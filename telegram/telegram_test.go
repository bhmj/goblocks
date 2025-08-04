package telegram

import (
	"testing"

	"github.com/bhmj/goblocks/log"
)

func TestSendMessage(t *testing.T) {
	cfg := Config{
		Endpoint: "/adminbot",
		Port:     80,
		BotToken: "bot_token",
		ChatID:   -1,
	}
	logger, _ := log.New("info", false)
	tg := New(cfg, logger)
	tg.Message("Plaintext: <b>Hi</b> <i>there</i> !").Send()
	tg.Message("MD: *Hi* _there_ \\!").Type(Markdown).Send()
	tg.Message("HTML: <b>Hi</b> <i>there</i> !").Type(HTML).Send()
}
