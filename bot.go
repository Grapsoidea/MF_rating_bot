package main

import (
	"encoding/json"
	"encoding/xml"
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

var groups = map[string]string{
	"МОС-151": "http://volsu.ru/activities/education/eduprogs/rating.php?plan=000000512&list=62&level=03&profile=0000000002&semestr=6",
	"ПМ-161":  "http://volsu.ru/activities/education/eduprogs/rating.php?plan=000000816&list=32&level=03&profile=&semestr=4",
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
	body, _ := ioutil.ReadAll(resp.Body)

	data := string(body)

	i := strings.Index(data, `<table class="progs">`)
	data = data[i:]
	i = strings.Index(data, `</table>`)
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
	//filename is the path to the json config file
	file, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		panic(err)
	}

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

	updates := bot.ListenForWebhook("/")

	go http.ListenAndServe(":8080", nil)
	fmt.Println("start listen :8080")

	// получаем все обновления из канала updates
	for update := range updates {
		re := regexp.MustCompile(`[А-Я][А-Я]+-\d\d\d`)
		group := re.FindString(strings.ToUpper(update.Message.Text))
		re = regexp.MustCompile(`\d\d\d\d\d\d`)
		recBook := re.FindString(update.Message.Text)

		if group != "" && recBook != "" {
			if url, ok := groups[group]; ok {
				table, err := getTable(url)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(
						update.Message.Chat.ID,
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
						update.Message.Chat.ID,
						"Извините, зачетка: "+recBook+" не найдена",
					))
				} else {
					for i = 1; i < len(table.Rows[0].Cols); i++ {

						re = regexp.MustCompile(`<div>.*</div>`)
						dis := re.FindString(table.Rows[0].Cols[i].Cell)
						dis = dis[5 : len(dis)-6]
						re = regexp.MustCompile(`\d+`)
						rat := re.FindString(table.Rows[n].Cols[i].Cell)

						bot.Send(tgbotapi.NewMessage(
							update.Message.Chat.ID,
							dis+":\n"+rat,
						))
					}
				}
			} else {
				bot.Send(tgbotapi.NewMessage(
					update.Message.Chat.ID,
					`Доступны только группы: ПМ-161, МОС-151`,
				))
			}
		} else {
			bot.Send(tgbotapi.NewMessage(
				update.Message.Chat.ID,
				`Введите свою группу и номер зач. книжки, например "МОС-151 123456"`,
			))
		}

	}
}