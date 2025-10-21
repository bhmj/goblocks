package telegram

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/bhmj/goblocks/log"
)

type ParseMode string

const (
	Plaintext ParseMode = ""
	HTML      ParseMode = "HTML"
	Markdown  ParseMode = "MarkdownV2"
)

type UserCallback func() (string, bool)

type tg struct {
	endpoint string
	msgID    atomic.Int64
	botToken string
	chatID   int64
	port     int
	sync.RWMutex
	callbacks map[int64][]UserCallback
	logger    log.MetaLogger
}

type Telegram interface {
	Message(message string) *Message
	Run(ctx context.Context) error
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

type Update struct {
	Message       *TelegramMessage `json:"message,omitempty"`
	CallbackQuery *CallbackQuery   `json:"callback_query,omitempty"`
}

type CallbackQuery struct {
	ID      string          `json:"id"`
	Message TelegramMessage `json:"message"`
	Data    string          `json:"data"`
}

type TelegramMessage struct { //nolint:revive
	MessageID int              `json:"message_id"`
	Text      string           `json:"text"`
	Chat      Chat             `json:"chat"`
	Entities  *[]MessageEntity `json:"entities"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type MessageEntity struct {
	Type          string `json:"type"`
	Offset        int    `json:"offset"`
	Length        int    `json:"length"`
	URL           string `json:"url,omitempty"`
	Language      string `json:"language,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}
