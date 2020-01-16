package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"liveCoding-api/util"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	ProjectPath string `json:"projectPath" bson:"project_path"`
}

// type Commands struct {
// 	Content string `json:"content" bson:"content"`
// }

type FileInfo struct {
	Code     string        `json:"code" bson:"code"`
	Lang     string        `json:"lang" bson:"lang"`
	Commands util.Commands `json:"commands" bson:"commands"`
}

type LiveResponse struct {
	ProjectPath string              `json:"projectPath" bson:"project_path"`
	ProjectName string              `json:"projectName" bson:"project_name"`
	Hash        string              `json:"hash" bson:"hash"`
	Time        int64               `json:"time" bson:"time"`
	ID          int                 `json:"id" bson:"id"`
	Files       map[string]FileInfo `json:"files" bson:"files"`
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
	// fmt.Println(message)
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

func dirwalk(dir string) (error, []string) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err, nil
	}
	var paths []string
	for _, file := range files {
		fileName := file.Name()
		filePathJoin := filepath.Join(dir, fileName)
		if file.IsDir() {
			err, tempPaths := dirwalk(filePathJoin)
			if err != nil {
				return err, nil
			}

			paths = append(paths, tempPaths...)
			continue
		}
		paths = append(paths, filePathJoin)
	}

	return nil, paths
}

func liveListRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "OPTIONS":
			CORSforOptions(&w)
			return
		case "GET":
			// requestBody, err := ioutil.ReadAll(r.Body)
			// defer r.Body.Close()
			// if err != nil {
			// 	responseErrorJSON(w, http.StatusInternalServerError, err.Error())
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

			pipeline := bson.A{
				bson.D{{"$group", bson.D{{"_id", "$project_path"}}}},
			}
			cur, err := liveCodingCollection.Aggregate(ctx, pipeline)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			projectsNameMap := []map[string]string{}

			err = cur.All(ctx, &projectsNameMap)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			projectsName := []string{}

			for _, value := range projectsNameMap {
				projectsName = append(projectsName, value["_id"])
			}

			// fmt.Println(projectsNameMap)

			// fmt.Println(requestBody)

			responseJSON(w, http.StatusOK, projectsName)
		default:
			responseErrorJSON(w, http.StatusMethodNotAllowed, "Sorry, only GET method is supported.")
			return
		}
	}
}

func liveRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "OPTIONS":
			CORSforOptions(&w)
			return
		case "POST":
			if r.Close == true {
				return
			}

			requestBody, err := ioutil.ReadAll(r.Body)
			defer r.Body.Close()
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}
			liveRequest := LiveRequest{ProjectPath: ""}

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

			// fmt.Println(liveRequest.ProjectPath)
			filter := bson.M{"project_path": liveRequest.ProjectPath}
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

			// fmt.Println(livesResponse)
			// err = cur.All(ctx, &livesResponse)
			// if err != nil {
			// 	responseErrorJSON(w, http.StatusInternalServerError, err.Error())
			// 	return
			// }

			// fmt.Println(livesResponse)

			// TODO: for の外に出す｡
			var repo *git.Repository
			if _, err := os.Stat(liveRequest.ProjectPath); os.IsNotExist(err) {
				repo, err = git.PlainInit(liveRequest.ProjectPath, false)
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}
			} else {
				repo, err = git.PlainOpen(liveRequest.ProjectPath)
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

			cmd := exec.Command("git", "stash")
			cmd.Dir = wt.Filesystem.Root()
			err = cmd.Run()
			if err != nil {
				fmt.Println(err, 111)
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			for i := 0; i < len(livesResponse); i++ {
				// fmt.Println(i)
				liveResponse := livesResponse[i]

				err = wt.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("master"),
				})
				if err != nil {
					fmt.Println(err, 222)
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				err = wt.Checkout(&git.CheckoutOptions{
					Hash: plumbing.NewHash(liveResponse.Hash),
				})
				if err != nil {
					fmt.Println(err, 333)
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				fileInfos, err := ioutil.ReadDir(liveRequest.ProjectPath)
				if err != nil {
					log.Fatal(err)
				}

				absPaths := []string{}

				for _, file := range fileInfos {
					fileName := file.Name()
					absPath := liveRequest.ProjectPath + "/" + fileName
					if file.IsDir() {
						if fileName == ".git" {
							continue
						}
						err, tempPaths := dirwalk(absPath)
						if err != nil {
							log.Fatal(err)
						}
						absPaths = append(absPaths, tempPaths...)
					} else {
						absPaths = append(absPaths, absPath)
					}
				}

				fileInfo := map[string]FileInfo{}

				for j := 0; j < len(absPaths); j++ {
					path := absPaths[j]

					bytes, err := ioutil.ReadFile(path)
					if err != nil {
						responseErrorJSON(w, http.StatusInternalServerError, err.Error())
						return
					}

					code := string(bytes)
					// fmt.Println(path, code, 1111)
					fileInfoStruct := FileInfo{}
					fileInfoStruct.Code = code
					// fileInfo[path] = FileInfo{Code: code}
					// fileInfo[path]

					baseName := filepath.Base(path)

					extention := filepath.Ext(baseName)

					plaintext := "plaintext"

					if baseName == ".cui.log" {
						fileInfoStruct.Lang = "bash"
					} else {

						// ex a.py -> py
						// extention := path[pos+1:]
						if len(extention) == 0 {
							fileInfoStruct.Lang = plaintext
						} else {
							if extention == ".py" {
								fileInfoStruct.Lang = "python"
							} else {
								fileInfoStruct.Lang = plaintext
							}
						}
					}

					// fmt.Println(code)

					err, commands := util.GetCommands(fileInfoStruct.Code, fileInfoStruct.Lang, baseName)
					if err != nil {

					}
					// fmt.Println(baseName, code, commands, 777)

					fileInfoStruct.Commands = commands

					fileInfo[path] = fileInfoStruct
				}

				err = wt.Checkout(&git.CheckoutOptions{
					Branch: plumbing.NewBranchReferenceName("master"),
				})
				if err != nil {
					fmt.Println(err, 777)
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				livesResponse[i].Files = fileInfo
			}
			// fmt.Println(livesResponse)
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
	liveListEndpointName := apiEndpointName + "/liveList"

	http.HandleFunc(liveEndpointName, liveRequest())
	http.HandleFunc(liveListEndpointName, liveListRequest())
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
