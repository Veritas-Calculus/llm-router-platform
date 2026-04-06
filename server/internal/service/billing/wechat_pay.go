package billing

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WechatPayService handles WeChat Pay Native payment processing.
type WechatPayService struct {
	cfg            config.WechatPayConfig
	frontendURL    string
	subRepo        repository.SubscriptionRepo
	txRepo         repository.TransactionRepo
	logger         *zap.Logger
	privateKey     *rsa.PrivateKey
	platformPubKey *rsa.PublicKey
}

// NewWechatPayService creates a new WeChat Pay service instance.
func NewWechatPayService(
	cfg config.WechatPayConfig,
	frontendURL string,
	subRepo repository.SubscriptionRepo,
	txRepo repository.TransactionRepo,
	logger *zap.Logger,
) *WechatPayService {
	svc := &WechatPayService{
		cfg:         cfg,
		frontendURL: frontendURL,
		subRepo:     subRepo,
		txRepo:      txRepo,
		logger:      logger,
	}
	if cfg.Enabled {
		if cfg.PrivateKey != "" {
			pk, err := parsePrivateKey(cfg.PrivateKey)
			if err != nil {
				logger.Error("failed to parse wechat pay private key", zap.Error(err))
			} else {
				svc.privateKey = pk
			}
		}
		if cfg.PlatformCertPEM != "" {
			pubKey, err := parsePublicKey(cfg.PlatformCertPEM)
			if err != nil {
				logger.Error("failed to parse wechat pay platform certificate", zap.Error(err))
			} else {
				svc.platformPubKey = pubKey
			}
		}
	}
	return svc
}

// CreateNativeOrder creates a WeChat Pay Native order and returns a QR code URL.
func (s *WechatPayService) CreateNativeOrder(ctx context.Context, userID uuid.UUID, amount float64, description string) (string, string, error) {
	if !s.cfg.Enabled {
		return "", "", fmt.Errorf("wechat pay is not enabled")
	}
	if s.privateKey == nil {
		return "", "", fmt.Errorf("wechat pay private key not configured")
	}

	orderNo := fmt.Sprintf("WX-%d-%s", time.Now().Unix(), uuid.New().String()[:8])
	amountCents := int64(amount * 100) // WeChat uses cents (分)

	reqBody := map[string]interface{}{
		"appid":        s.cfg.AppID,
		"mchid":        s.cfg.MchID,
		"description":  description,
		"out_trade_no": orderNo,
		"notify_url":   s.cfg.NotifyURL,
		"amount": map[string]interface{}{
			"total":    amountCents,
			"currency": "CNY",
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	url := "https://api.mch.weixin.qq.com/v3/pay/transactions/native"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	// Sign the request with WECHATPAY2-SHA256-RSA2048
	nonce := generateNonce()
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signStr := fmt.Sprintf("POST\n/v3/pay/transactions/native\n%s\n%s\n%s\n", timestamp, nonce, string(bodyBytes))

	signature, err := s.sign(signStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign request: %w", err)
	}

	authHeader := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		s.cfg.MchID, nonce, timestamp, s.cfg.SerialNo, signature)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("wechat pay API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("wechat pay API error", zap.Int("status", resp.StatusCode), zap.String("body", string(respBody)))
		return "", "", fmt.Errorf("wechat pay API returned status %d", resp.StatusCode)
	}

	var result struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Create order record
	order := &models.Order{
		OrgID:         userID,
		OrderNo:       orderNo,
		Amount:        amount,
		Status:        "pending",
		PaymentMethod: "wechat",
	}
	if err := s.subRepo.CreateOrder(ctx, order); err != nil {
		s.logger.Error("failed to create wechat order record", zap.Error(err))
	}

	s.logger.Info("wechat pay native order created", zap.String("order_no", orderNo), zap.Float64("amount", amount))
	return result.CodeURL, orderNo, nil
}

// verifyNotifySignature verifies the HTTP signature on a WeChat Pay notification
// using the platform certificate public key per WeChat Pay APIv3 spec.
func (s *WechatPayService) verifyNotifySignature(body []byte, headers http.Header) error {
	if s.platformPubKey == nil {
		return fmt.Errorf("wechat pay platform certificate not configured, cannot verify notification signature")
	}

	timestamp := headers.Get("Wechatpay-Timestamp")
	nonce := headers.Get("Wechatpay-Nonce")
	signature := headers.Get("Wechatpay-Signature")

	if timestamp == "" || nonce == "" || signature == "" {
		return fmt.Errorf("missing required wechat pay signature headers")
	}

	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, string(body))
	h := sha256.New()
	h.Write([]byte(message))
	hashed := h.Sum(nil)

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("failed to decode wechat pay signature: %w", err)
	}

	if err := rsa.VerifyPKCS1v15(s.platformPubKey, crypto.SHA256, hashed, sigBytes); err != nil {
		return fmt.Errorf("wechat pay notification signature verification failed: %w", err)
	}
	return nil
}

// HandleNotify processes WeChat Pay async notification.
func (s *WechatPayService) HandleNotify(body []byte, headers http.Header) (string, error) {
	if err := s.verifyNotifySignature(body, headers); err != nil {
		return "", err
	}

	// Parse the notification envelope
	var notification struct {
		ID           string `json:"id"`
		CreateTime   string `json:"create_time"`
		ResourceType string `json:"resource_type"`
		EventType    string `json:"event_type"`
		Resource     struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(body, &notification); err != nil {
		return "", fmt.Errorf("failed to parse notification: %w", err)
	}

	// Decrypt the resource using APIv3 key with AES-256-GCM
	plaintext, err := s.decryptResource(
		notification.Resource.Ciphertext,
		notification.Resource.Nonce,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt resource: %w", err)
	}

	var txResult struct {
		OutTradeNo  string `json:"out_trade_no"`
		TradeState  string `json:"trade_state"`
		TransID     string `json:"transaction_id"`
		Amount      struct {
			Total    int    `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(plaintext, &txResult); err != nil {
		return "", fmt.Errorf("failed to parse decrypted result: %w", err)
	}

	ctx := context.Background()

	// Idempotency check
	order, err := s.subRepo.GetOrderByNo(ctx, txResult.OutTradeNo)
	if err != nil {
		return "", fmt.Errorf("order not found: %s", txResult.OutTradeNo)
	}
	if order.Status == "paid" {
		s.logger.Info("wechat order already fulfilled", zap.String("order_no", sanitize.LogValue(txResult.OutTradeNo)))
		return txResult.OutTradeNo, nil
	}

	if txResult.TradeState == "SUCCESS" {
		order.Status = "paid"
		order.ExternalID = txResult.TransID
		_ = s.subRepo.UpdateOrder(ctx, order)

		// Credit user balance
		amount := float64(txResult.Amount.Total) / 100.0
		if err := s.subRepo.UpdateUserBalance(ctx, order.OrgID, amount, "recharge", "Credit Top-up via WeChat Pay", txResult.OutTradeNo); err != nil {
			s.logger.Error("failed to credit user balance for wechat payment", zap.Error(err), zap.String("order_no", sanitize.LogValue(txResult.OutTradeNo)))
			return "", err
		}
		s.logger.Info("wechat pay order fulfilled", zap.String("order_no", sanitize.LogValue(txResult.OutTradeNo)), zap.Float64("amount", amount))
	}

	return txResult.OutTradeNo, nil
}

// QueryOrderStatus queries the payment status of an order from WeChat Pay API.
func (s *WechatPayService) QueryOrderStatus(ctx context.Context, orderNo string) (string, error) {
	if !s.cfg.Enabled || s.privateKey == nil {
		return "unknown", fmt.Errorf("wechat pay not configured")
	}

	url := fmt.Sprintf("https://api.mch.weixin.qq.com/v3/pay/transactions/out-trade-no/%s?mchid=%s", orderNo, s.cfg.MchID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "unknown", err
	}

	nonce := generateNonce()
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	path := fmt.Sprintf("/v3/pay/transactions/out-trade-no/%s?mchid=%s", orderNo, s.cfg.MchID)
	signStr := fmt.Sprintf("GET\n%s\n%s\n%s\n\n", path, timestamp, nonce)
	signature, err := s.sign(signStr)
	if err != nil {
		return "unknown", err
	}

	authHeader := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		s.cfg.MchID, nonce, timestamp, s.cfg.SerialNo, signature)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "unknown", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		TradeState string `json:"trade_state"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "unknown", err
	}
	return result.TradeState, nil
}

// sign creates a SHA256 with RSA signature.
func (s *WechatPayService) sign(message string) (string, error) {
	h := sha256.New()
	h.Write([]byte(message))
	hashed := h.Sum(nil)
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// decryptResource decrypts AES-256-GCM encrypted notification resource.
func (s *WechatPayService) decryptResource(ciphertext, nonce, associatedData string) ([]byte, error) {
	key := []byte(s.cfg.APIv3Key)
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	nonceBytes := []byte(nonce)
	additionalData := []byte(associatedData)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonceBytes, ciphertextBytes, additionalData)
}

// parsePrivateKey parses a PEM-encoded RSA private key.
func parsePrivateKey(keyPEM string) (*rsa.PrivateKey, error) {
	keyPEM = strings.TrimSpace(keyPEM)
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 format
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA")
	}
	return rsaKey, nil
}

// generateNonce generates a random 32-character hex string.
func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
