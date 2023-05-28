package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Env                string
	EmailProviderToken string
	Query              string `yaml:"query"`
	Filepath           string `yaml:"filepath"`
	Batchsize          int    `yaml:"batchsize"`
	EmailFrom          string `yaml:"emailFrom"`
	EmailSubject       string `yaml:"emailSubject"`
	EmailTextBody      string `yaml:"emailTextBody"`
	EmailHtmlBody      string `yaml:"emailHtmlBody"`
	EmailMessageStream string `yaml:"emailMessageStream"`
}

type Price struct {
	Price BtcUah `json:"btc_uah"`
}

type BtcUah struct {
	Sell string `json:"sell"`
}

type EmailFormat struct {
	From          string `json:"From"`
	To            string `json:"To"`
	Subject       string `json:"Subject"`
	TextBody      string `json:"TextBody"`
	HtmlBody      string `json:"HtmlBody"`
	MessageStream string `json:"MessageStream"`
}

var emails []string
var config Config

func GetPrice() float64 {
	resp, err := http.Get(config.Query)
	if err != nil {
		// handle error
	}
	defer resp.Body.Close()
	var price Price
	json.NewDecoder(resp.Body).Decode(&price)
	if err != nil {
		log.Fatalf("impossible to marshall teacher: %s", err)
	}
	value, err := strconv.ParseFloat(price.Price.Sell, 32)
	return value
}

func InitConfig() Config {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatal(err)
	}

	config.Env = os.Getenv("ENV")
	config.EmailProviderToken = os.Getenv("EMAIL_PROVIDER_TOKEN")

	if config.Env == "dev" {
		log.Println(config)
	}
	return config
}

func IsEmailSubscribed(filePath string, email string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if email == scanner.Text() {
			return true
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return false
}

func AddEmailToFile(filePath string, email string) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	if _, err = f.WriteString(email + "\n"); err != nil {
		log.Fatal(err)
	}
}

func SendBatch(emails []string, rate float64) {

	var body []EmailFormat
	for _, email := range emails {
		emailFormatted := EmailFormat{
			config.EmailFrom,
			email,
			config.EmailSubject,
			fmt.Sprintf(config.EmailTextBody, rate),
			fmt.Sprintf(config.EmailHtmlBody, rate),
			config.EmailMessageStream}
		body = append(body, emailFormatted)
	}
	bytesBody, _ := json.Marshal(body)

	r, _ := http.NewRequest("POST", "https://api.postmarkapp.com/email/batch", bytes.NewBuffer(bytesBody))
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("X-Postmark-Server-Token", "qwertyuiop")
	r.Header.Add("Accept", "application/json")

	if config.Env == "dev" {
		log.Println(r)
	} else {
		client := &http.Client{}
		res, err := client.Do(r)
		if err != nil {
			log.Fatal(err)
		}
		res.Body.Close()
	}
}

func Add(c *gin.Context) {
	email, exist := c.GetPostForm("email")
	if exist {
		email = strings.ToLower(email)
		if IsEmailSubscribed(config.Filepath, email) {
			c.AbortWithStatus(409)
		} else {
			AddEmailToFile(config.Filepath, email)
		}
	} else {
		c.AbortWithStatus(400)
	}
}

func SendToAll(c *gin.Context) {
	file, err := os.Open(config.Filepath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	price := GetPrice()
	var currentBatch []string
	i := 0
	for scanner.Scan() {
		i++
		currentBatch = append(currentBatch, scanner.Text())

		if i >= config.Batchsize {
			SendBatch(currentBatch, price)
			currentBatch = nil
			i = 0
		}
	}
	if i > 0 {
		SendBatch(currentBatch, price)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func Rate(c *gin.Context) {
	c.IndentedJSON(200, GetPrice())
}

func main() {
	config = InitConfig()
	router := gin.Default()

	router.GET("/rate", Rate)
	router.POST("/subscribe", Add)
	router.POST("/sendEmails", SendToAll)

	router.Run(":8080")
}
