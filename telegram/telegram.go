package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bhmj/goblocks/log"
)

type Config struct {
	Endpoint string `yaml:"endpoint" description:"Telegram callback endpoint"`
	Port     int    `yaml:"port" description:"Telegram callback port"`
	BotToken string `yaml:"bot_token" description:"Telegram bot token"`
	ChatID   int64  `yaml:"chat_id" description:"Default target chat ID"`
}

func New(cfg Config, logger log.MetaLogger) Telegram {
	return &tg{
		endpoint:  cfg.Endpoint,
		botToken:  cfg.BotToken,
		chatID:    cfg.ChatID,
		port:      cfg.Port,
		logger:    logger,
		callbacks: make(map[int64][]UserCallback),
	}
}

func (t *tg) Run(ctx context.Context) error {
	http.HandleFunc(t.endpoint, t.WebhookHandler)

	t.logger.Info("TG webhook listening", log.Int("port", t.port))

	errCh := make(chan error, 1)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", t.port), nil) //nolint:gosec
		if !errors.Is(err, http.ErrServerClosed) {
			t.logger.Info("TG webhook server closed", log.Error(err))
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (t *tg) Message(message string) *Message {
	return t.createMessage(message)
}

func (t *tg) WebhookHandler(_ http.ResponseWriter, r *http.Request) {
	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		t.logger.Error("Error decoding update", log.Error(err))
		return
	}

	if update.CallbackQuery != nil { //nolint:nestif
		data := update.CallbackQuery.Data
		parts := strings.Split(data, ":")
		if len(parts) == 2 { //nolint:mnd
			msgID, _ := strconv.Atoi(parts[0])
			iBtn, _ := strconv.Atoi(parts[1])
			//
			done := "Done"
			appendix, success := t.callback(int64(msgID), iBtn)
			if success {
				message := update.CallbackQuery.Message.Text
				err := t.appendMessage(
					r.Context(),
					update.CallbackQuery.Message.Chat.ID,
					update.CallbackQuery.Message.MessageID,
					message,
					appendix,
					update.CallbackQuery.Message.Entities,
				)
				if err != nil {
					t.logger.Error("appendMessage", log.Error(err))
				}
			} else {
				done = "Error"
			}
			err := t.answerCallbackQuery(r.Context(), update.CallbackQuery.ID, done, !success)
			if err != nil {
				t.logger.Error("answerCallbackQuery", log.Error(err))
			}
		}
	}
}

func (t *tg) AddCallback(messageID int64, fn UserCallback) {
	t.Lock()
	defer t.Unlock()
	a, found := t.callbacks[messageID]
	if !found {
		a = make([]UserCallback, 0)
	}
	a = append(a, fn)
	t.callbacks[messageID] = a
}

func (t *tg) SendMessage(ctx context.Context, message string, chatID int64, parseMode string, replyMarkup *InlineKeyboardMarkup) error {
	if chatID == 0 {
		chatID = t.chatID
	}
	if t.botToken == "" || chatID == 0 {
		return nil
	}

	payload := map[string]any{
		"chat_id": chatID,
		"text":    message,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	return t.send(ctx, "sendMessage", payload)
}

func (t *tg) createMessage(message string) *Message {
	nextID := t.msgID.Add(1)
	return &Message{
		id:     nextID,
		sender: t,
		text:   message,
	}
}

func (t *tg) callback(messageID int64, iButton int) (string, bool) {
	t.Lock()
	defer t.Unlock()
	a, found := t.callbacks[messageID]
	if !found {
		t.logger.Error("callback for message not found")
		return "", false
	}
	fn := a[iButton]
	delete(t.callbacks, messageID)

	if fn != nil {
		return fn()
	}
	t.logger.Error("fn is empty")
	return "", true // empty callback
}

func (t *tg) send(ctx context.Context, method string, payload map[string]any) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.botToken, method)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("non-200 response: %s, Body: %s", resp.Status, string(buf))
	}
	return nil
}

func (t *tg) answerCallbackQuery(ctx context.Context, queryID string, message string, alert bool) error {
	payload := map[string]any{
		"callback_query_id": queryID,
	}
	if message != "" {
		payload["text"] = message
	}
	if alert {
		payload["show_alert"] = true
	}

	return t.send(ctx, "answerCallbackQuery", payload)
}

func (t *tg) appendMessage(ctx context.Context, chatID int64, messageID int, text, appendix string, entities *[]MessageEntity) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text + appendix,
	}
	if entities != nil {
		payload["entities"] = entities
	}
	return t.send(ctx, "editMessageText", payload)
}
