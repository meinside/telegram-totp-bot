package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbot "github.com/meinside/telegram-bot-go"
	"github.com/meinside/version-go"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	// Telegram Bot commands
	botCommandStart      = "/start"
	botCommandNewTOTP    = "/new"
	botCommandListTOTP   = "/list"
	botCommandDeleteTOTP = "/del"
	botCommandTOTP       = "/otp"
	botCommandHelp       = "/help"
	botCommandCancel     = "/cancel"
	botCommandPrivacy    = "/privacy"

	botCommandDescriptionNewTOTP    = "Create a new TOTP."
	botCommandDescriptionListTOTP   = "List all your TOTPs."
	botCommandDescriptionTOTP       = "Generate a TOTP code."
	botCommandDescriptionDeleteTOTP = "Delete your TOTP."
	botCommandDescriptionHelp       = "Display help message."
	botCommandDescriptionPrivacy    = "Display privacy policy."

	// Telegram Bot polling interval seconds
	pollingIntervalSeconds = 1

	githubPageURL = "https://github.com/meinside/telegram-totp-bot"
)

func Run(telegramAPIToken, dbFilepath string) {
	bot := tgbot.NewClient(telegramAPIToken)
	if res := bot.GetMe(); res.Ok {
		// delete webhook before polling
		if res := bot.DeleteWebhook(false); !res.Ok {
			log.Fatalf("Failed to delete webhook: %s", *res.Description)
		}

		// set bot commands
		_ = bot.SetMyCommands([]tgbot.BotCommand{
			{
				Command:     botCommandTOTP,
				Description: botCommandDescriptionTOTP,
			},
			{
				Command:     botCommandNewTOTP,
				Description: botCommandDescriptionNewTOTP,
			},
			{
				Command:     botCommandListTOTP,
				Description: botCommandDescriptionListTOTP,
			},
			{
				Command:     botCommandDeleteTOTP,
				Description: botCommandDescriptionDeleteTOTP,
			},
			{
				Command:     botCommandPrivacy,
				Description: botCommandDescriptionPrivacy,
			},
			{
				Command:     botCommandHelp,
				Description: botCommandDescriptionHelp,
			},
		}, nil)

		// open database
		db, err := gorm.Open(sqlite.Open(dbFilepath), &gorm.Config{})
		if err != nil {
			log.Fatalf("Failed to connect database: %s", err)
		}

		// migrate database
		if err = db.AutoMigrate(&TOTP{}, &TempTOTP{}, &EditableMessageCache{}); err != nil {
			log.Fatalf("Failed to migrate database: %s", err)
		}

		// set update handler functions
		bot.SetMessageHandler(func(b *tgbot.Bot, update tgbot.Update, message tgbot.Message, edited bool) {
			handleMessage(b, db, message)
		})
		bot.SetCallbackQueryHandler(func(b *tgbot.Bot, update tgbot.Update, callbackQuery tgbot.CallbackQuery) {
			handleCallbackQuery(b, db, callbackQuery)
		})

		// start polling messages
		log.Printf("Starting TOTP bot (@%s)...", res.Result.FirstName)
		bot.StartPollingUpdates(0, pollingIntervalSeconds, func(bot *tgbot.Bot, update tgbot.Update, err error) {
			if err != nil {
				log.Printf("Failed to poll updates: %s", err.Error())
			}
		})
	}
}

// parse "/command number1,number2,number3..." and extract numbers from it
func parseCallbackQueryData(command, data string) (result []uint64, err error) {
	result = []uint64{}

	tokens := strings.Split(strings.TrimSpace(strings.Replace(data, command, "", 1)), ",")
	for _, token := range tokens {
		number, e := strconv.ParseUint(token, 10, 64)
		result = append(result, number)

		if e != nil {
			err = e
		}
	}

	return result, err
}

// message handler
func handleMessage(bot *tgbot.Bot, db *gorm.DB, message tgbot.Message) {
	if message.HasText() {
		text := *message.Text
		chatID := message.Chat.ID
		userID := message.From.ID

		if strings.HasPrefix(text, "/") {
			switch text {
			case botCommandNewTOTP:
				sendMessage(bot, chatID, "Input name for your OTP:", false)
			case botCommandListTOTP:
				bot.SendChatAction(chatID, tgbot.ChatActionTyping, tgbot.OptionsSendChatAction{})

				if totps, err := ListTOTPs(db, userID); err == nil {
					if len(totps) == 0 {
						sendMessage(bot, chatID, "You have no TOTP registered yet.", true)
					} else {
						names := []string{}
						for _, t := range totps {
							names = append(names, "• "+t.Name)
						}
						sendMessage(bot, chatID, fmt.Sprintf("Your TOTPs:\n\n%s", strings.Join(names, "\n")), true)
					}
				} else {
					sendError(bot, chatID, fmt.Sprintf("Failed to list your TOTPs: %s", err), false)
				}
			case botCommandDeleteTOTP:
				bot.SendChatAction(chatID, tgbot.ChatActionTyping, tgbot.OptionsSendChatAction{})

				if totps, err := ListTOTPs(db, userID); err == nil {
					if len(totps) == 0 {
						sendMessage(bot, chatID, "You have no TOTP registered yet.", true)
					} else {
						kvs := map[string]string{}

						var inlineKeyboard = [][]tgbot.InlineKeyboardButton{}
						var cancelableID uint
						if cancelableID, err = SaveEditableMessage(db, userID); err == nil {
							for _, t := range totps {
								kvs[t.Name] = fmt.Sprintf("%s %d,%d", botCommandDeleteTOTP, t.ID, cancelableID)
							}
							inlineKeyboard = tgbot.NewInlineKeyboardButtonsAsRowsWithCallbackData(kvs)

							// cancel command
							cancelData := fmt.Sprintf("%s %d", botCommandCancel, cancelableID)
							inlineKeyboard = append(inlineKeyboard, []tgbot.InlineKeyboardButton{
								{
									Text:         "Cancel",
									CallbackData: &cancelData,
								},
							})
						}

						if res := bot.SendMessage(chatID, "Select TOTP to delete:", tgbot.OptionsSendMessage{}.SetReplyMarkup(tgbot.InlineKeyboardMarkup{
							InlineKeyboard: inlineKeyboard,
						})); res.Ok {
							if err = UpdateEditableMessage(db, cancelableID, res.Result.MessageID); err != nil {
								log.Printf("Failed to update cancelable message: %s", err)
							}
						} else {
							log.Printf("Failed to send message: %s", *res.Description)
						}
					}
				} else {
					sendError(bot, chatID, fmt.Sprintf("Failed to list your TOTPs: %s", err), false)
				}
			case botCommandTOTP:
				bot.SendChatAction(chatID, tgbot.ChatActionTyping, tgbot.OptionsSendChatAction{})

				if totps, err := ListTOTPs(db, userID); err == nil {
					if len(totps) == 0 {
						sendMessage(bot, chatID, "You have no TOTP registered yet.", true)
					} else {
						kvs := map[string]string{}

						var inlineKeyboard = [][]tgbot.InlineKeyboardButton{}
						var cancelableID uint
						if cancelableID, err = SaveEditableMessage(db, userID); err == nil {
							for _, t := range totps {
								kvs[t.Name] = fmt.Sprintf("%s %d,%d", botCommandTOTP, t.ID, cancelableID)
							}
							inlineKeyboard = tgbot.NewInlineKeyboardButtonsAsRowsWithCallbackData(kvs)

							// cancel command
							cancelData := fmt.Sprintf("%s %d", botCommandCancel, cancelableID)
							inlineKeyboard = append(inlineKeyboard, []tgbot.InlineKeyboardButton{
								{
									Text:         "Cancel",
									CallbackData: &cancelData,
								},
							})
						}

						if res := bot.SendMessage(chatID, "Select TOTP to generate OTP:", tgbot.OptionsSendMessage{}.SetReplyMarkup(tgbot.InlineKeyboardMarkup{
							InlineKeyboard: inlineKeyboard,
						})); res.Ok {
							if err = UpdateEditableMessage(db, cancelableID, res.Result.MessageID); err != nil {
								log.Printf("Failed to update cancelable message: %s", err)
							}
						} else {
							log.Printf("Failed to send message: %s", *res.Description)
						}
					}
				} else {
					sendError(bot, chatID, fmt.Sprintf("Failed to list your TOTPs: %s", err), false)
				}
			case botCommandStart:
				fallthrough
			case botCommandHelp:
				sendMessage(bot, chatID, helpMessage(), true)
			case botCommandPrivacy:
				sendMessage(bot, chatID, privacyPolicyMessage(), true)
			default:
				sendError(bot, chatID, fmt.Sprintf("No such command: %s", text), true)
			}
		} else {
			bot.SendChatAction(chatID, tgbot.ChatActionTyping, tgbot.OptionsSendChatAction{})

			if temp, _ := GetTempTOTP(db, userID); temp != nil {
				if temp.Name == nil {
					name := text

					if err := SaveTempTOTP(db, userID, temp.ID, &name); err == nil {
						sendMessage(bot, chatID, fmt.Sprintf("Input secret for your TOTP `%s`:", name), false)
					} else {
						sendError(bot, chatID, fmt.Sprintf("Failed to save temporary TOTP: %s", err), false)
					}
				} else {
					name := *temp.Name
					secret := text

					if _, err := SaveTOTP(db, userID, name, secret); err == nil {
						if err := DeleteTempTOTP(db, userID, temp.ID); err != nil {
							sendError(bot, chatID, fmt.Sprintf("Failed to delete temporary TOTP with id: %d", temp.ID), false)
						}

						messageID := message.MessageID
						if res := bot.DeleteMessage(chatID, messageID); !res.Ok {
							log.Printf("Failed to delete user's message with secret: %s", *res.Description)
						}

						sendMessage(bot, chatID, fmt.Sprintf("Your TOTP `%s` was successfully created.", name), true)
					} else {
						sendError(bot, chatID, fmt.Sprintf("Failed to save TOTP `%s`", name), true)
					}
				}
			} else {
				sendMessage(bot, chatID, helpMessage(), true)
			}
		}
	} else {
		chatID := message.Chat.ID

		sendMessage(bot, chatID, "Invalid message type", true)
	}
}

// callback query handler
func handleCallbackQuery(bot *tgbot.Bot, db *gorm.DB, query tgbot.CallbackQuery) {
	chatID := query.Message.Chat.ID

	data := query.Data
	if data != nil {
		bot.SendChatAction(chatID, tgbot.ChatActionTyping, tgbot.OptionsSendChatAction{})

		userID := query.From.ID

		switch {
		case strings.HasPrefix(*data, botCommandDeleteTOTP):
			ids, err := parseCallbackQueryData(botCommandDeleteTOTP, *data)
			totpID := uint(ids[0])
			editableID := uint(ids[1])
			if err == nil {
				if err = DeleteTOTP(db, userID, totpID); err == nil {
					if editableMessage, err := GetEditableMessage(db, editableID); err == nil {
						bot.EditMessageText("Your TOTP was successfully deleted.", tgbot.OptionsEditMessageText{}.SetIDs(chatID, editableMessage.MessageID))

						// delete editable message cache
						if err := DeleteEditableMessage(db, editableID); err != nil {
							log.Printf("Failed to delete editable message cache: %s", err)
						}
					}
				} else {
					sendMessage(bot, chatID, fmt.Sprintf("Failed to delete your TOTP: %s", err), false)
				}
			} else {
				sendMessage(bot, chatID, fmt.Sprintf("Invalid callback query data: %s", *query.Data), false)
			}
		case strings.HasPrefix(*data, botCommandTOTP):
			ids, err := parseCallbackQueryData(botCommandTOTP, *data)
			totpID := uint(ids[0])
			editableID := uint(ids[1])
			if err == nil {
				if generated, err := GenerateTOTP(db, userID, totpID); err == nil {
					// update message with generated value
					if editableMessage, err := GetEditableMessage(db, editableID); err == nil {
						bot.EditMessageText(generated, tgbot.OptionsEditMessageText{}.SetIDs(chatID, editableMessage.MessageID))

						// delete editable message cache
						if err := DeleteEditableMessage(db, editableID); err != nil {
							log.Printf("Failed to delete editable message cache: %s", err)
						}
					}
				} else {
					sendError(bot, chatID, fmt.Sprintf("Failed to generate OTP: %s", err), false)
				}
			} else {
				sendMessage(bot, chatID, fmt.Sprintf("Invalid callback query data: %s", *query.Data), false)
			}
		case strings.HasPrefix(*data, botCommandHelp):
			sendMessage(bot, chatID, helpMessage(), true)
		case strings.HasPrefix(*data, botCommandCancel):
			ids, err := parseCallbackQueryData(botCommandCancel, *data)
			cancelableID := uint(ids[0])
			if err == nil {
				if cancelableMessage, err := GetEditableMessage(db, cancelableID); err == nil {
					// update message as 'canceled'
					bot.EditMessageText("Canceled", tgbot.OptionsEditMessageText{}.SetIDs(chatID, cancelableMessage.MessageID))

					// delete editable message cache
					if err := DeleteEditableMessage(db, cancelableID); err != nil {
						log.Printf("Failed to delete editable message cache: %s", err)
					}
				}
			} else {
				sendMessage(bot, chatID, fmt.Sprintf("Invalid callback query data: %s", *query.Data), false)
			}
		default:
			sendError(bot, chatID, fmt.Sprintf("Invalid callback query data for this context: %s", *query.Data), false)
		}
	} else {
		sendMessage(bot, chatID, "Callback query data is empty.", false)
	}
}

func helpMessage() string {
	return fmt.Sprintf(`Usage:

%s : %s
%s : %s
%s : %s
%s : %s
%s : %s
%s : %s

* version: %s

* Source code: %s`,
		botCommandNewTOTP, botCommandDescriptionNewTOTP,
		botCommandTOTP, botCommandDescriptionTOTP,
		botCommandListTOTP, botCommandDescriptionListTOTP,
		botCommandDeleteTOTP, botCommandDescriptionDeleteTOTP,
		botCommandPrivacy, botCommandDescriptionPrivacy,
		botCommandHelp, botCommandDescriptionHelp,
		version.Minimum(),
		githubPageURL,
	)
}

func privacyPolicyMessage() string {
	return fmt.Sprintf(`Privacy Policy:

%s/raw/master/PRIVACY.md`, githubPageURL)
}

func sendMessage(bot *tgbot.Bot, chatID int64, message string, withButtons bool) {
	options := tgbot.OptionsSendMessage{}
	if withButtons {
		options = tgbot.OptionsSendMessage{}.SetReplyMarkup(keyboardMarkups())
	}

	result := bot.SendMessage(chatID, message, options)

	if !result.Ok {
		if result.Description != nil {
			log.Printf("Failed to send message: %s", *result.Description)
		} else {
			log.Printf("Failed to send message.")
		}
	}
}

func sendError(bot *tgbot.Bot, chatID int64, message string, withButtons bool) {
	log.Println(message)

	sendMessage(bot, chatID, message, withButtons)
}

func keyboardMarkups() tgbot.ReplyKeyboardMarkup {
	return tgbot.NewReplyKeyboardMarkup([][]tgbot.KeyboardButton{
		{
			tgbot.KeyboardButton{Text: botCommandTOTP},
		},
		{
			tgbot.KeyboardButton{Text: botCommandNewTOTP},
			tgbot.KeyboardButton{Text: botCommandListTOTP},
			tgbot.KeyboardButton{Text: botCommandDeleteTOTP},
		},
		{

			tgbot.KeyboardButton{Text: botCommandPrivacy},
			tgbot.KeyboardButton{Text: botCommandHelp},
		},
	}).SetResizeKeyboard(true).
		SetOneTimeKeyboard(true).
		SetSelective(true)
}
