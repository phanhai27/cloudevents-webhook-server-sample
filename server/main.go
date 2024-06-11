package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"main/sms_events"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NdoleStudio/httpsms-go"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/golang-jwt/jwt/v5"
)

func authorize(authorizationString string) bool {
	authorization := strings.Split(authorizationString, " ")
	tokenString := ""
	if len(authorization) == 2 {
		tokenString = authorization[1] // authorization[0] == "Bearer"
	} else if len(authorization) == 1 {
		tokenString = authorization[0]
	}

	_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte("Signing Key"), nil
	})

	if err != nil {
		log.Printf("[ERROR] Authorize HttpSms fail: %v\n", err)
		return false
	}

	return true
}

func receive(event cloudevents.Event) {
	log.Printf("receive event: %s\n", event)

	if event.Type() == sms_events.EventTypeMessagePhoneReceived && event.DataContentType() == cloudevents.ApplicationJSON {
		received_event := httpsms.Message{}
		err := json.Unmarshal(event.Data(), &received_event)
		if err != nil {
			log.Println("[ERROR] Unmarshal Payload: ", err)
			return
		}

		textEncrypted := ""
		textNormal := ""
		decrypted := false
		if received_event.Encrypted {
			textEncrypted = received_event.Content

			client := httpsms.New(httpsms.WithAPIKey("API KEY"))
			textNormal, err = client.Cipher.Decrypt("ENCRYPTION KEY", textEncrypted)
			if err != nil {
				log.Println("[ERROR] Decrypt Content: ", err)
				log.Println("[ERROR] Please check all secret keys!")
			} else {
				decrypted = true
			}
		} else {
			textNormal = received_event.Content
		}

		// Results
		log.Printf("from: %s \n", received_event.Contact)
		log.Printf("to: %s \n", received_event.Owner)
		log.Printf("encrypted status: %t \n", received_event.Encrypted)

		if !received_event.Encrypted || decrypted {
			log.Println("sms content: ", textNormal)
		} else {
			log.Println("sms content encrypted: ", textEncrypted)
		}
	}
}

func main() {
	errch := make(chan error)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ctx := context.Background()
	p, err := cloudevents.NewHTTP()
	if err != nil {
		log.Fatalf("failed to create protocol: %s", err.Error())
	}

	httpEvents, err := cloudevents.NewHTTPReceiveHandler(ctx, p, receive)
	if err != nil {
		log.Fatalf("failed to create handler: %s", err.Error())
	}

	// HTTP Server serve Restful API
	httpMux := http.NewServeMux()

	// Handle webhook request which has a payload of the cloudevents format
	httpMux.HandleFunc("/webhook/", func(w http.ResponseWriter, r *http.Request) {
		authorizationString := r.Header.Get("Authorization")
		if authorizationString != "" { // optional
			log.Println("[INFO] Cloudevents webhook - Authorization header: ", authorizationString)
			if !authorize(authorizationString) {
				return
			}
		}

		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("[ERROR] Cloudevents webhook - Read request body: ", err)
			return
		}

		event := cloudevents.NewEvent()
		err = json.Unmarshal(bytes, &event)
		if err != nil {
			log.Println("[ERROR] Cloudevents webhook - Unmarshal body to event: ", err)
			return
		}

		receive(event)
	})

	// Handle cloudevents requests from cloudevents.NewClientHTTP() [Not Webhook]
	httpMux.Handle("/", httpEvents)

	httpAddress := "0.0.0.0:8080"
	httpServer := http.Server{
		Addr:    httpAddress,
		Handler: httpMux,
	}

	go func() {
		log.Println("================= HTTP Server Starting at", httpAddress, "=================")
		if err := httpServer.ListenAndServe(); err != nil {
			errch <- err
		}
	}()

	for {
		select {
		case <-stop:
			ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
			httpServer.Shutdown(ctx)
			return
		case err := <-errch:
			log.Println("[ERROR] HTTP Server: ", err)
			return
		}
	}
}
