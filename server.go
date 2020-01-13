package main

import (
	"context"
	"encoding/json"
	"flag"
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

type Config struct {
	Schema string `json:"schema"`
	Host   string `json:"host"`
	Port   string `json:"port"`
}

type Configs struct {
	Test    Config `json:"test"`
	Product Config `json:"product"`
}

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
			liveRequest := LiveRequest{ID: -1}

			err = json.Unmarshal(requestBody, &liveRequest)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			// if liveRequest.ID == -1 {
			// 	responseErrorJSON(w, http.StatusInternalServerError, "ID not found.")
			// 	return
			// }

			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}
			defer client.Disconnect(ctx)

			liveCodingCollection := client.Database("liveCoding").Collection("commit")

			fmt.Println(liveRequest.ID)
			filter := bson.M{}
			cur, err := liveCodingCollection.Find(ctx, filter)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			livesResponse := LivesResponse{}
			err = cur.All(ctx, &livesResponse)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			fmt.Println(livesResponse)
			// err = cur.All(ctx, &livesResponse)
			// if err != nil {
			// 	responseErrorJSON(w, http.StatusInternalServerError, err.Error())
			// 	return
			// }

			// fmt.Println(livesResponse)

			for i := 0; i < len(livesResponse); i++ {
				liveResponse := livesResponse[i]

				// TODO: for の外に出す｡
				var repo *git.Repository
				if _, err := os.Stat(liveResponse.ProjectName); os.IsNotExist(err) {
					repo, err = git.PlainInit(liveResponse.ProjectName, false)
					if err != nil {
						responseErrorJSON(w, http.StatusInternalServerError, err.Error())
						return
					}
				} else {
					repo, err = git.PlainOpen(liveResponse.ProjectName)
					if err != nil {
						responseErrorJSON(w, http.StatusInternalServerError, err.Error())
						return
					}
				}

				wt, err := repo.Worktree()
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				err = wt.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("master"),
				})
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				err = wt.Checkout(&git.CheckoutOptions{
					Hash: plumbing.NewHash(liveResponse.Hash),
				})
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				files := map[string]string{}
				walkErr := filepath.Walk(liveResponse.ProjectName,
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

				err = wt.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("master"),
				})
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				if walkErr != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}
				fmt.Println(111, files)
				livesResponse[i].Files = files
			}
			fmt.Println(livesResponse)
			responseJSON(w, http.StatusOK, livesResponse)
			return

		default:
			responseErrorJSON(w, http.StatusMethodNotAllowed, "Sorry, only POST method is supported.")
			return
		}

		// unreach
	}
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fmt.Println("args are invalid.")
		return
	}

	configsJSON, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	configs := Configs{}

	err = json.Unmarshal(configsJSON, &configs)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	envType := flag.Arg(0)

	const envTypeTest = "test"
	const envTypeProduct = "product"

	config := Config{}
	if envType == envTypeTest {
		config = configs.Test
	} else if envType == envTypeProduct {
		config = configs.Product
	} else {
		log.Println("config-type is invalid.")
		os.Exit(1)
	}

	apiEndpointName := "/api"
	liveEndpointName := apiEndpointName + "/live"
	http.HandleFunc(liveEndpointName, liveRequest())
	schema := config.Schema
	host := config.Host
	port := config.Port
	addr := host + ":" + port
	fmt.Println("LISTEN: ", schema+"://"+addr)
	// 証明書の作成参考: https://ozuma.hatenablog.jp/entry/20130511/1368284304
	if envType == envTypeTest {
		err = http.ListenAndServeTLS(addr, "cert_key/test/cert.pem", "cert_key/test/key.pem", nil)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	} else if envType == envTypeProduct {
		err = http.ListenAndServeTLS(addr, "cert_key/product/cert.pem", "cert_key/product/key.pem", nil)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	} else {
		log.Println("config-type is invalid.")
		os.Exit(1)
	}
}
