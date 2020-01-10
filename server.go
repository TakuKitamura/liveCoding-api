package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type LiveRequest struct {
	ID int `json:"id" bson:"id"`
}

type LiveResponse struct {
	ProjectName string            `json:"projectName" bson:"project_name"`
	Hash        string            `json:"hash" bson:"hash"`
	Time        int64             `json:"time" bson:"time"`
	ID          int               `json:"id" bson:"id"`
	Files       map[string]string `json:"files" bson:"files"`
}

type LivesResponse []LiveResponse

type ErrorResponse struct {
	Message string `json:"message"`
}

type ErrorsResponse []ErrorResponse

func responseJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func responseErrorJSON(w http.ResponseWriter, code int, message string) {
	fmt.Println(message)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	errorsResponse := ErrorsResponse{
		ErrorResponse{
			Message: message,
		},
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorsResponse)
}

func CORSforOptions(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
	(*w).WriteHeader(204)
}

func liveRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "OPTIONS":
			CORSforOptions(&w)
			return
		case "POST":
			requestBody, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}
			liveRequest := LiveRequest{}

			err = json.Unmarshal(requestBody, &liveRequest)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}
			defer client.Disconnect(ctx)

			liveCodingCollection := client.Database("liveCoding").Collection("commit")

			fmt.Println(liveRequest.ID)
			filter := bson.M{"id": liveRequest.ID}
			r := liveCodingCollection.FindOne(ctx, filter)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			liveResponse := LiveResponse{}

			err = r.Decode(&liveResponse)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			// err = cur.All(ctx, &livesResponse)
			// if err != nil {
			// 	responseErrorJSON(w, http.StatusInternalServerError, err.Error())
			// 	return
			// }

			fmt.Println(liveResponse)

			files := map[string]string{}
			err = filepath.Walk(liveResponse.ProjectName,
				func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if path != liveResponse.ProjectName {
						if strings.HasPrefix(path, liveResponse.ProjectName+"/.git") == false {
							file, err := os.Stat(path)
							if err != nil {
								return err
							}
							if file.Mode().IsRegular() {
								var r *git.Repository
								if _, err := os.Stat(liveResponse.ProjectName); os.IsNotExist(err) {
									r, err = git.PlainInit(liveResponse.ProjectName, false)
									if err != nil {
										return err
									}
								} else {
									r, err = git.PlainOpen(liveResponse.ProjectName)
									if err != nil {
										return err
									}
								}
								w, err := r.Worktree()
								if err != nil {
									return err
								}

								err = w.Checkout(&git.CheckoutOptions{
									Hash: plumbing.NewHash(liveResponse.Hash),
								})
								if err != nil {
									return err
								}
								bytes, err := ioutil.ReadFile(path)
								if err != nil {
									return err
								}
								fmt.Println(path)

								files[path] = string(bytes)
							}
						}
					}
					return nil
				})
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			liveResponse.Files = files

			responseJSON(w, http.StatusOK, liveResponse)
			return

		default:
			responseErrorJSON(w, http.StatusMethodNotAllowed, "Sorry, only POST method is supported.")
			return
		}

		// unreach
	}
}

func main() {
	apiEndpointName := "/api"
	liveEndpointName := apiEndpointName + "/live"
	http.HandleFunc(liveEndpointName, liveRequest())
	err := http.ListenAndServe("localhost:3000", nil)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
