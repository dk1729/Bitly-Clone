package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/codegangsta/negroni"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
	"github.com/unrolled/render"
)

var rabbitmq_server = "52.37.250.82"
var rabbitmq_port = "5672"
var rabbitmq_queue = "bitly"
var rabbitmq_user = "guest"
var rabbitmq_pass = "guest"

var mysql_connect = "cmpe281:root@tcp(10.0.1.82:3306)/go"

func NewServer() *negroni.Negroni {
	formatter := render.New(render.Options{
		IndentJSON: true,
	})
	n := negroni.Classic()
	mx := mux.NewRouter()
	initRoutes(mx, formatter)
	n.UseHandler(mx)
	return n
}

func init() {

	db, err := sql.Open("mysql", mysql_connect)
	if err != nil {
		log.Fatal(err)
	} else {
		var (
			id        int
			url       string
			shortlink string
			count     int
		)
		rows, err := db.Query("select * from urls where id = ?", 1)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			err := rows.Scan(&id, &url, &shortlink, &count)
			if err != nil {
				log.Fatal(err)
			}
			log.Println(id, url, shortlink)
		}
		err = rows.Err()
		if err != nil {
			log.Fatal(err)
		}
	}
	defer db.Close()

}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

// API Routes
func initRoutes(mx *mux.Router, formatter *render.Render) {
	mx.HandleFunc("/", createLink(formatter)).Methods("POST")
}

func createLink(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if url := req.PostFormValue("url"); url != "" {

			var shortlink string
			if sv := req.PostFormValue("sv"); sv != "" {
				shortlink = sv
			}

			m := make(map[string]interface{})
			m["url"] = url
			m["shortlink"] = shortlink

			p, _ := serialize(m)

			queue_send(p)
			queue_receive()
			newURL := "34.222.46.211:3001/" + shortlink
			formatter.JSON(w, http.StatusOK, newURL)
		}
	}
}

type Message map[string]interface{}

func serialize(msg Message) ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	err := encoder.Encode(msg)
	return b.Bytes(), err
}

func deserialize(b []byte) (Message, error) {
	var msg Message
	buf := bytes.NewBuffer(b)
	decoder := json.NewDecoder(buf)
	err := decoder.Decode(&msg)
	return msg, err
}

func queue_send(message []byte) {
	conn, err := amqp.Dial("amqp://" + rabbitmq_user + ":" + rabbitmq_pass + "@" + rabbitmq_server + ":" + rabbitmq_port + "/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		rabbitmq_queue, // name
		false,          // durable
		false,          // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	failOnError(err, "Failed to declare a queue")

	body := message
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	log.Printf(" [x] Sent %s", body)
	failOnError(err, "Failed to publish a message")
}

func queue_receive() {
	conn, err := amqp.Dial("amqp://" + rabbitmq_user + ":" + rabbitmq_pass + "@" + rabbitmq_server + ":" + rabbitmq_port + "/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		rabbitmq_queue, // name
		false,          // durable
		false,          // delete when usused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	var shortlink string
	go func() {
		for d := range msgs {
			db, err := sql.Open("mysql", mysql_connect)
			defer db.Close()
			if err != nil {
				log.Fatal(err)
			} else {
				fmt.Println("Trying to store in db")
				m, _ := deserialize(d.Body)
				url := m["url"]
				shortlink = m["shortlink"].(string)
				rows, err := db.Query("insert into urls(url,shortlink,count) values (?,?,?);", url, shortlink, 1)
				if err != nil {
					fmt.Println("An error has occured")
					fmt.Println(err)
				} else {
					fmt.Println(rows)
				}
				fmt.Println("stored in db")
			}
			fmt.Print("Your new url is : ")
			fmt.Println(shortlink)
		}
	}()

}
