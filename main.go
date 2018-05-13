package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"gopkg.in/telegram-bot-api.v4"
)

type Configuration struct {
	BotToken   string
	WebhookURL string
}

var groups map[string]string

func MainHandler(resp http.ResponseWriter, _ *http.Request) {
	resp.Write([]byte("Hi there! I'm MF_telegram_bot!"))
}

type XMLTable struct {
	Rows []struct {
		Cols []struct {
			Cell string `xml:",innerxml"`
		} `xml:"td"`
	} `xml:"tr"`
}

func getTable(url string) (*XMLTable, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	data := string(body)

	i := strings.Index(data, `<table class="progs">`)
	if i == -1 {
		err = errors.New("Bad strings.Index")
		return nil, err
	}
	data = data[i:]

	i = strings.Index(data, `</table>`)
	if i == -1 {
		err = errors.New("Bad strings.Index")
		return nil, err
	}
	data = data[:i+8]

	table := new(XMLTable)
	decoder := xml.NewDecoder(strings.NewReader(data))
	decoder.Entity = xml.HTMLEntity
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Strict = false

	err = decoder.Decode(&table)
	if err != nil {
		return nil, err
	}

	return table, nil
}

func main() {
	configuration := new(Configuration)

	file, err := os.Open("groups.json")
	if err != nil {
		panic(err)
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&groups)

	if err != nil {
		panic(err)
	}

	configuration.BotToken = os.Getenv("BotToken")
	configuration.WebhookURL = os.Getenv("WebhookURL")

	bot, err := tgbotapi.NewBotAPI(configuration.BotToken)
	if err != nil {
		panic(err)
	}

	// bot.Debug = true
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(configuration.WebhookURL))
	if err != nil {
		panic(err)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)

	http.HandleFunc("/", MainHandler)

	go http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	fmt.Println("start listen :" + os.Getenv("PORT"))

	// получаем все обновления из канала updates
	for update := range updates {
		var text string
		var uMessage int64
		if update.CallbackQuery != nil {
			text = update.CallbackQuery.Data
			uMessage = update.CallbackQuery.Message.Chat.ID
		} else {
			text = update.Message.Text
			uMessage = update.Message.Chat.ID
		}

		re := regexp.MustCompile(`[А-Я][А-Я]+-\d\d\d`)
		group := re.FindString(strings.ToUpper(text))
		re = regexp.MustCompile(`\d\d\d\d\d\d`)
		recBook := re.FindString(text)
		re = regexp.MustCompile(`СЕМЕСТР=[1-8]`)
		semestr := re.FindString(strings.ToUpper(text))

		if group != "" && recBook != "" {
			if url, ok := groups[group]; ok {
				newUrl := url
				if semestr != "" {
					newUrl += "&semestr=" + semestr[len(semestr)-1:]
				} else {
					semestr = "СЕМЕСТР ТЕКУЩИЙ"
				}

				table, err := getTable(newUrl)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(
						uMessage,
						"Извините, произошла ошибка",
					))
				}
				n := 0
				i := 0
				for i = 1; i < len(table.Rows); i++ {
					if table.Rows[i].Cols[0].Cell == recBook {
						n = i
						break
					}
				}
				if i == len(table.Rows) {
					bot.Send(tgbotapi.NewMessage(
						uMessage,
						"Извините, зачетка: "+recBook+" не найдена",
					))
				} else {
					bot.Send(tgbotapi.NewMessage(
						uMessage,
						"Зачетная книжка - "+recBook+", "+strings.ToLower(semestr)+":\n",
					))
					for i = 1; i < len(table.Rows[0].Cols); i++ {

						re := regexp.MustCompile(`<div>.*</div>`)
						disA := re.FindAllString(table.Rows[0].Cols[i].Cell, -1)
						dis := ""
						if len(disA) > 0 {
							dis = disA[0][5 : len(disA[0])-6]
							if len(disA) == 3 {
								if disA[2][5:len(disA[2])-6] != "" {
									dis += " (" + disA[2][5:len(disA[2])-6] + ")"
								}
							}
						} else {
							dis = "Что-то с названием предмета не так...("
						}

						re = regexp.MustCompile(`\d+`)
						rat := re.FindString(table.Rows[n].Cols[i].Cell)

						bot.Send(tgbotapi.NewMessage(
							uMessage,
							dis+":\n"+rat,
						))
					}
					msg := tgbotapi.NewMessage(uMessage, "Нажмите, чтобы обновить этот список")
					btn := tgbotapi.NewInlineKeyboardButtonData("Обновить", group+" "+recBook+" "+semestr)
					var row []tgbotapi.InlineKeyboardButton
					row = append(row, btn)
					keyboard := tgbotapi.InlineKeyboardMarkup{}
					keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, row)
					msg.ReplyMarkup = keyboard
					bot.Send(msg)

				}
			} else {
				bot.Send(tgbotapi.NewMessage(
					uMessage,
					`Извините, ваша группа не найдена: возможно она пока не доступна или вы ошиблись)`,
				))
			}
		} else {
			bot.Send(tgbotapi.NewMessage(
				uMessage,
				`Введите свою группу и номер зач. книжки, например "МОС-123 123456" (чтобы узнать баллы за n-ый семестр, допишите "семестр=n")`,
			))
		}

	}
}
