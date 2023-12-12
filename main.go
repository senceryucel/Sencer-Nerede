// Sencer Nerede - Main Server
// December 2023, Sencer Yucel

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"

	"time"

	"github.com/dgrijalva/jwt-go"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	redis "github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

var signKey = []byte(os.Getenv("JWT_SECRET"))

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	HttpPort string `json:"http_port"`
}

var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{
	Addr:     "redis:6379",
	Password: "",
	DB:       0,
})

// get current date and time
func getCurrentDateTime() string {
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		fmt.Println("Error:", err)
		return "01_01_23-00:00:01"
	}
	return time.Now().In(loc).Format("01_02_06-15:04:05")
}

func generateJWT() (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["authorized"] = true
	claims["user"] = os.Getenv("JWT_USER")
	claims["exp"] = time.Now().Add(time.Minute * 30).Unix()

	tokenString, err := token.SignedString(signKey)

	if err != nil {
		log.Println("Error in JWT token generation")
		return "", err
	}

	return tokenString, nil
}

func setCorsHeaders(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", os.Getenv("DOMAIN"))
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

/*
********************************
WEBSOCKET OPERATIONS
********************************
*/
var clients = make(map[*websocket.Conn]bool)

var broadcast = make(chan []byte)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// handle incoming ws connection
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil) // Upgrades HTTP connection to ws
	if err != nil {
		log.Fatal(err)
		return
	}
	defer ws.Close()

	clients[ws] = true

	log.Printf("New connection established from %s\n", r.RemoteAddr)

	// Fetch all keys
	keys, err := rdb.Keys(ctx, "*").Result()
	if err != nil {
		log.Fatal("Error fetching keys from Redis:", err)
		return
	}

	// Sort the keys
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	// Iterate over sorted keys and send to ws
	for _, key := range keys {
		data, err := getList(rdb, key)
		if err != nil {
			log.Fatal("Error fetching data from Redis for key", key, ":", err)
			continue
		}

		// Each set of three items in the list are lat, lon
		for i := 0; i < len(data); i += 3 {
			if i+1 < len(data) {
				locationJSON, _ := json.Marshal(map[string]string{
					"lat":       data[i],
					"lon":       data[i+1],
					"timestamp": key,
				})

				if err := ws.WriteMessage(websocket.TextMessage, locationJSON); err != nil {
					log.Fatal("Error sending message:", err)
					return
				}
			}
		}

	}

	// Read messages from ws
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			delete(clients, ws)
			break
		}
	}
}

// send message to the broadcast channel
func broadcastToWebSockets(message []byte) {
	broadcast <- message
}

// handle messages received on the broadcast channel
func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Fatal("websocket err:", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

/*
********************************
END OF WEBSOCKET OPERATIONS
********************************
*/

/*
********************************
MQTT OPERATIONS
********************************
*/
// MQTT message handler. (equivalent to on_message in Python)
var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	// fmt.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	re := regexp.MustCompile(`"latitude":([\d.]+),"longitude":([\d.]+)`)

	matches := re.FindStringSubmatch(string(msg.Payload()))
	fmt.Println(len(matches))
	if len(matches) != 3 {
		log.Fatal("Invalid message format")
		return
	}

	currentDateTime := getCurrentDateTime()

	// Inserting latitude, longitude into Redis as a list with key datetime.
	insertList(rdb, currentDateTime, []string{matches[1], matches[2]})

	// Create a map with the parsed location data and include the timestamp
	location := map[string]string{
		"lat":       matches[1],
		"lon":       matches[2],
		"timestamp": currentDateTime,
	}
	locationJSON, _ := json.Marshal(location)
	broadcastToWebSockets(locationJSON)
}

// MQTT connect handler (equivalent to on_connect in Python).
var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Printf("MQTT Connection successful") // Prints a message on successful connection.
}

// MQTT connection lost handler (equivalent to on_disconnect in Python).
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("MQTT connection lost: %v", err)
}

// subscribe to an MQTT topic
func sub(client mqtt.Client, topic string, qos byte) {
	token := client.Subscribe(topic, qos, nil)
	token.Wait()
	log.Printf("Subscribed to topic %s with QoS %d\n", topic, qos)
}

func sendLocationViaMQTT(client mqtt.Client, loc Location) {
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Println(token.Error())
		return
	}

	// Construct the message (example: simple JSON)
	msg, err := json.Marshal(loc)
	if err != nil {
		log.Println("Error marshaling location data:", err)
		return
	}

	// Publish
	token := client.Publish(os.Getenv("MQTT_TOPIC"), 0, false, msg)
	token.Wait()
}

/*
********************************
END OF MQTT OPERATIONS
********************************
*/

/*
********************************
REDIS DB OPERATIONS
********************************
*/
func insertList(rdb *redis.Client, key string, values []string) error {
	err := rdb.RPush(ctx, key, values).Err()
	if err != nil {
		return err
	}
	return nil
}

func getList(rdb *redis.Client, key string) ([]string, error) {
	val, err := rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	return val, nil
}

/*
********************************
END OF REDIS DB OPERATIONS
********************************
*/

func main() {
	logFile, err := os.OpenFile("/app/all.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.Printf("\n------Starting Server------")

	configFile, err := ioutil.ReadFile("/app/config.json")
	if err != nil {
		log.Fatal("Error reading config file:", err)
		os.Exit(1)
	}
	var config Config
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatal("Error parsing config file:", err)
		os.Exit(1)
	}

	go handleMessages()
	http.HandleFunc("/ws", handleConnections)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(os.Getenv("MQTT_HOST"))
	opts.SetClientID(os.Getenv("MQTT_CLIENT_NAME"))
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
		os.Exit(1)
	}

	sub(client, os.Getenv("MQTT_TOPIC"), 2) // client, topic, qos

	http.HandleFunc("/api/authenticate", func(w http.ResponseWriter, r *http.Request) {
		setCorsHeaders(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		var creds Credentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// TODO
		adminUsername := os.Getenv("ADMIN_USERNAME")
		adminPassword := os.Getenv("ADMIN_PASSWORD")

		if creds.Username == adminUsername && creds.Password == adminPassword {
			fmt.Println("Valid credentials")
			tokenString, err := generateJWT()
			if err != nil {
				http.Error(w, "Error generating token", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
		} else {
			fmt.Println("Invalid credentials")
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		}
	})

	http.HandleFunc("/api/location", func(w http.ResponseWriter, r *http.Request) {
		setCorsHeaders(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		var loc Location
		err := json.NewDecoder(r.Body).Decode(&loc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		sendLocationViaMQTT(client, loc)

		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("/var/www/html"))))

	http.HandleFunc("/nerede", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/var/www/html/nerede.html")
	})

	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "/var/www/html/update.html")
	})

	// starts a new goroutine.
	go func() {
		var http_port = config.HttpPort
		log.Print("Starting server on :" + http_port)
		err := http.ListenAndServe(":"+http_port, nil)
		if err != nil {
			log.Fatal("Failed to start HTTP server:", err)
		}
	}()

	fmt.Println("Server is up and running")
	select {}
}
