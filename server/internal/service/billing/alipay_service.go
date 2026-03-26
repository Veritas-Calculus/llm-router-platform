package billing

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	alipayGateway        = "https://openapi.alipay.com/gateway.do"
	alipaySandboxGateway = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
)

// AlipayService handles Alipay payment processing.
type AlipayService struct {
	cfg         config.AlipayConfig
	frontendURL string
	subRepo     repository.SubscriptionRepo
	txRepo      repository.TransactionRepo
	logger      *zap.Logger
	privateKey  *rsa.PrivateKey
	alipayPubKey *rsa.PublicKey
}

// NewAlipayService creates a new Alipay service instance.
func NewAlipayService(
	cfg config.AlipayConfig,
	frontendURL string,
	subRepo repository.SubscriptionRepo,
	txRepo repository.TransactionRepo,
	logger *zap.Logger,
) *AlipayService {
	svc := &AlipayService{
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
				logger.Error("failed to parse alipay private key", zap.Error(err))
			} else {
				svc.privateKey = pk
			}
		}
		if cfg.AlipayPublicKey != "" {
			pubKey, err := parsePublicKey(cfg.AlipayPublicKey)
			if err != nil {
				logger.Error("failed to parse alipay public key", zap.Error(err))
			} else {
				svc.alipayPubKey = pubKey
			}
		}
	}
	return svc
}

// CreatePreCreateOrder creates an Alipay precreate order and returns a QR code URL.
func (s *AlipayService) CreatePreCreateOrder(ctx context.Context, userID uuid.UUID, amount float64, description string) (string, string, error) {
	if !s.cfg.Enabled {
		return "", "", fmt.Errorf("alipay is not enabled")
	}
	if s.privateKey == nil {
		return "", "", fmt.Errorf("alipay private key not configured")
	}

	orderNo := fmt.Sprintf("ALI-%d-%s", time.Now().Unix(), uuid.New().String()[:8])

	// Build business content
	bizContent := map[string]interface{}{
		"out_trade_no": orderNo,
		"total_amount": fmt.Sprintf("%.2f", amount),
		"subject":      description,
	}
	bizContentJSON, _ := json.Marshal(bizContent)

	// Build common request parameters
	params := map[string]string{
		"app_id":      s.cfg.AppID,
		"method":      "alipay.trade.precreate",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  s.cfg.NotifyURL,
		"biz_content": string(bizContentJSON),
	}

	// Sign the parameters
	sign, err := s.signParams(params)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign request: %w", err)
	}
	params["sign"] = sign

	// Build form-encoded body
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	gateway := alipayGateway
	if s.cfg.IsSandbox {
		gateway = alipaySandboxGateway
	}

	resp, err := http.PostForm(gateway, values)
	if err != nil {
		return "", "", fmt.Errorf("alipay API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		AlipayTradePrecreateResponse struct {
			Code   string `json:"code"`
			Msg    string `json:"msg"`
			QRCode string `json:"qr_code"`
		} `json:"alipay_trade_precreate_response"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse alipay response: %w", err)
	}

	if result.AlipayTradePrecreateResponse.Code != "10000" {
		s.logger.Error("alipay precreate failed",
			zap.String("code", result.AlipayTradePrecreateResponse.Code),
			zap.String("msg", result.AlipayTradePrecreateResponse.Msg))
		return "", "", fmt.Errorf("alipay error: %s - %s",
			result.AlipayTradePrecreateResponse.Code,
			result.AlipayTradePrecreateResponse.Msg)
	}

	// Create order record
	order := &models.Order{
		OrgID:         userID,
		OrderNo:       orderNo,
		Amount:        amount,
		Status:        "pending",
		PaymentMethod: "alipay",
	}
	if err := s.subRepo.CreateOrder(ctx, order); err != nil {
		s.logger.Error("failed to create alipay order record", zap.Error(err))
	}

	s.logger.Info("alipay precreate order created", zap.String("order_no", orderNo), zap.Float64("amount", amount))
	return result.AlipayTradePrecreateResponse.QRCode, orderNo, nil
}

// HandleNotify processes Alipay async notification.
func (s *AlipayService) HandleNotify(formValues url.Values) (string, error) {
	// Verify signature
	if s.alipayPubKey != nil {
		if !s.verifyNotifySign(formValues) {
			return "", fmt.Errorf("invalid alipay notification signature")
		}
	}

	tradeStatus := formValues.Get("trade_status")
	orderNo := formValues.Get("out_trade_no")
	tradeNo := formValues.Get("trade_no")

	ctx := context.Background()

	// Idempotency check
	order, err := s.subRepo.GetOrderByNo(ctx, orderNo)
	if err != nil {
		return "", fmt.Errorf("order not found: %s", orderNo)
	}
	if order.Status == "paid" {
		s.logger.Info("alipay order already fulfilled", zap.String("order_no", sanitize.LogValue(orderNo)))
		return orderNo, nil
	}

	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		order.Status = "paid"
		order.ExternalID = tradeNo
		_ = s.subRepo.UpdateOrder(ctx, order)

		// Credit user balance
		if err := s.subRepo.UpdateUserBalance(ctx, order.OrgID, order.Amount, "recharge", "Credit Top-up via Alipay", orderNo); err != nil {
			s.logger.Error("failed to credit user balance for alipay payment", zap.Error(err), zap.String("order_no", sanitize.LogValue(orderNo)))
			return "", err
		}
		s.logger.Info("alipay order fulfilled", zap.String("order_no", sanitize.LogValue(orderNo)), zap.Float64("amount", order.Amount))
	}

	return orderNo, nil
}

// QueryOrderStatus queries the payment status of an order from Alipay API.
func (s *AlipayService) QueryOrderStatus(ctx context.Context, orderNo string) (string, error) {
	if !s.cfg.Enabled || s.privateKey == nil {
		return "unknown", fmt.Errorf("alipay not configured")
	}

	bizContent := map[string]string{"out_trade_no": orderNo}
	bizJSON, _ := json.Marshal(bizContent)

	params := map[string]string{
		"app_id":      s.cfg.AppID,
		"method":      "alipay.trade.query",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizJSON),
	}

	sign, err := s.signParams(params)
	if err != nil {
		return "unknown", err
	}
	params["sign"] = sign

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	gateway := alipayGateway
	if s.cfg.IsSandbox {
		gateway = alipaySandboxGateway
	}

	resp, err := http.PostForm(gateway, values)
	if err != nil {
		return "unknown", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Response struct {
			TradeStatus string `json:"trade_status"`
		} `json:"alipay_trade_query_response"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "unknown", err
	}
	return result.Response.TradeStatus, nil
}

// signParams creates an RSA2 signature for request parameters.
func (s *AlipayService) signParams(params map[string]string) (string, error) {
	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sign string
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(params[k])
	}

	h := sha256.New()
	h.Write([]byte(buf.String()))
	hashed := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// verifyNotifySign verifies the signature of an Alipay notification.
func (s *AlipayService) verifyNotifySign(values url.Values) bool {
	sign := values.Get("sign")
	if sign == "" {
		return false
	}

	// Sort parameters and build sign string
	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(values.Get(k))
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false
	}

	h := sha256.New()
	h.Write([]byte(buf.String()))
	hashed := h.Sum(nil)

	return rsa.VerifyPKCS1v15(s.alipayPubKey, crypto.SHA256, hashed, sigBytes) == nil
}

// parsePublicKey parses a PEM or base64 encoded RSA public key.
func parsePublicKey(keyPEM string) (*rsa.PublicKey, error) {
	keyPEM = strings.TrimSpace(keyPEM)
	// If it doesn't look like a PEM, wrap it
	if !strings.HasPrefix(keyPEM, "-----") {
		keyPEM = "-----BEGIN PUBLIC KEY-----\n" + keyPEM + "\n-----END PUBLIC KEY-----"
	}
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA public key")
	}
	return rsaPub, nil
}
