package telegram

import "fmt"

type Message struct {
	id        int64
	sender    *tg
	text      string
	chatID    int64
	parseMode ParseMode
	buttons   []string
}

func (m *Message) Type(typ ParseMode) *Message {
	m.parseMode = typ
	return m
}

func (m *Message) ToChat(chatID int64) *Message {
	m.chatID = chatID
	return m
}

func (m *Message) WithButton(text string, fn UserCallback) *Message {
	m.buttons = append(m.buttons, text)
	m.sender.AddCallback(m.id, fn)
	return m
}

func (m *Message) Send() error {
	var replyMarkup *InlineKeyboardMarkup = nil
	if len(m.buttons) > 0 {
		buttons := []InlineKeyboardButton{}
		for i, btn := range m.buttons {
			cData := fmt.Sprintf("%d:%d", m.id, i)
			buttons = append(buttons, InlineKeyboardButton{Text: btn, CallbackData: cData})
		}
		replyMarkup = &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{buttons}}
	}
	return m.sender.SendMessage(
		m.text,
		m.chatID,
		string(m.parseMode),
		replyMarkup,
	)
}
