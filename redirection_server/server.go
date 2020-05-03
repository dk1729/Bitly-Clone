/*
	Gumball API in Go (Version 4)
	Uses MySQL
*/

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

var rabbitmq_server = "localhost"
var rabbitmq_port = "5672"
var rabbitmq_queue = "gumball"
var rabbitmq_user = "guest"
var rabbitmq_pass = "guest"

/*
	Go's SQL Package:
		Tutorial: http://go-database-sql.org/index.html
		Reference: https://golang.org/pkg/database/sql/
*/

var mysql_connect = "root:rootroot@tcp(127.0.0.1:3306)/go"

//var mysql_connect = "root:cmpe281@tcp(mysql:3306)/cmpe281"

// NewServer configures and returns a Server.
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

// Init MySQL DB Connection

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
	mx.HandleFunc("/{id}", redirector(formatter)).Methods("GET")
}

func redirector(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		shortlink := mux.Vars(req)["id"]
		fmt.Println(shortlink)

		queue_send(shortlink)
		url := queue_receive()

		fmt.Println("Finally redirecting to : ", url)
		http.Redirect(w, req, url, 301)
	}
}

// API Gumball Machine Handler
func getUrls(formatter *render.Render) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		var (
			id         int
			url1       string
			shortlink1 string
			count1     int
		)
		db, err := sql.Open("mysql", mysql_connect)
		defer db.Close()
		if err != nil {
			log.Fatal(err)
		} else {
			rows, _ := db.Query("select * from urls")
			defer rows.Close()
			for rows.Next() {
				rows.Scan(&id, &url1, &shortlink1, &count1)
				fmt.Println(id, url1, shortlink1, count1)
			}
		}
		result := shortner{
			ID:        id,
			url:       url1,
			shortlink: shortlink1,
			count:     count1,
		}

		fmt.Println("URL Machine:", result)
		formatter.JSON(w, http.StatusOK, result)
	}
}

func queue_send(message string) {
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

func queue_receive() string {
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

	str_ch := make(chan string)
	go func() {
		for d := range msgs {
			var shortlink string
			shortlink = string(d.Body)
			resp, _ := http.Get("http://localhost:3500/" + shortlink)

			if resp.StatusCode == 200 {
				fmt.Println("Getting url from cache..")
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				str_ch <- result["message"].(string)
			} else {

				fmt.Println("Getting url from SQL..")
				fmt.Println("Shortlink is : ", shortlink)
				db, err := sql.Open("mysql", mysql_connect)
				defer db.Close()
				if err != nil {
					log.Fatal(err)
				} else {
					rows, err := db.Query("select url,count from urls where shortlink = ?", shortlink)
					var url string
					var count int
					if err != nil {
						log.Fatal(err)
					}
					defer rows.Close()
					for rows.Next() {
						err := rows.Scan(&url, &count)
						stmt, _ := db.Prepare("update urls set count = ? where shortlink = ?")
						if err != nil {
							log.Fatal(err)
						}
						_, err = stmt.Exec(count+1, shortlink)
						if err != nil {
							log.Fatal(err)
						}

						if count+1 > 5 {
							values := map[string]string{"message": url}
							jsonValue, _ := json.Marshal(values)
							resp1, _ := http.Post("http://localhost:9002/api/"+shortlink, "application/json", bytes.NewBuffer(jsonValue))
							fmt.Println(resp1.Body)
						}
						if err != nil {
							log.Fatal(err)
						}
						str_ch <- url
						fmt.Println(url)
					}

					err = rows.Err()
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}()
	url := <-str_ch
	return url
}
