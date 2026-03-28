package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"

	"go.uber.org/zap"
)

// processSSEStream reads Server-Sent Events from the response body and sends StreamChunks to the channel.
func processSSEStream(ctx context.Context, body io.ReadCloser, chunks chan<- StreamChunk, logger *zap.Logger) {
	defer close(chunks)
	defer func() { _ = body.Close() }()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			if logger != nil {
				logger.Debug("stream cancelled by context")
			}
			return
		default:
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				select {
				case chunks <- StreamChunk{Done: true}:
				case <-ctx.Done():
				}
				return
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			select {
			case chunks <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}
}
