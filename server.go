package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"liveCoding-api/util"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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

type LiveUpload struct {
	AssignProjectName   string `json:"assignProjectName" bson:"assign_project_name"`
	OriginalProjectName string `json:"originalProjectName" bson:"original_project_name"`
	HostedProjectPath   string `json:"hostedProjectPath" bson:"hosted_project_path"`
}

type LiveUploadResponse struct {
	URL string `json:"url"`
}

type LiveUploadsResponse []LiveUploadResponse

type Commit struct {
	ProjectPath string `bson:"project_path"`
	ProjectName string `bson:"project_name"`
	projectPath string `bson:"project_path"`
	Hash        string `bson:"hash"`
	Time        int64  `bson:"time"`
	ID          int    `bson:"id"`
	// Files       map[string]string `bson:"files"`
}

type Commits []Commit

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

func randomText(length int) string {
	rand.Seed(time.Now().UnixNano())
	const charSet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charSet[rand.Intn(len(charSet))]
	}

	return string(b)
}

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func untar(r io.Reader, dir string) (err error) {
	t0 := time.Now()
	nFiles := 0
	madeDir := map[string]bool{}
	defer func() {
		td := time.Since(t0)
		if err == nil {
			// log.Printf("extracted tarball into %s: %d files, %d dirs (%v)", dir, nFiles, len(madeDir), td)
		} else {
			log.Printf("error extracting tarball into %s after %d files, %d dirs, %v: %v", dir, nFiles, len(madeDir), td, err)
		}
	}()
	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("requires gzip-compressed body: %v", err)
	}
	tr := tar.NewReader(zr)
	loggedChtimesError := false
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("tar reading error: %v", err)
			return fmt.Errorf("tar error: %v", err)
		}
		if !validRelPath(f.Name) {
			return fmt.Errorf("tar contained invalid name error %q", f.Name)
		}
		rel := filepath.FromSlash(f.Name)
		abs := filepath.Join(dir, rel)

		fi := f.FileInfo()
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			// Make the directory. This is redundant because it should
			// already be made by a directory entry in the tar
			// beforehand. Thus, don't check for errors; the next
			// write will fail with the same error.
			dir := filepath.Dir(abs)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			n, err := io.Copy(wf, tr)
			if closeErr := wf.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %v", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
			}
			modTime := f.ModTime
			if modTime.After(t0) {
				// Clamp modtimes at system time. See
				// golang.org/issue/19062 when clock on
				// buildlet was behind the gitmirror server
				// doing the git-archive.
				modTime = t0
			}
			if !modTime.IsZero() {
				if err := os.Chtimes(abs, modTime, modTime); err != nil && !loggedChtimesError {
					// benign error. Gerrit doesn't even set the
					// modtime in these, and we don't end up relying
					// on it anywhere (the gomote push command relies
					// on digests only), so this is a little pointless
					// for now.
					log.Printf("error changing modtime: %v (further Chtimes errors suppressed)", err)
					loggedChtimesError = true // once is enough
				}
			}
			nFiles++
		case mode.IsDir():
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
			madeDir[abs] = true
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return nil
}

func validRelativeDir(dir string) bool {
	if strings.Contains(dir, `\`) || path.IsAbs(dir) {
		return false
	}
	dir = path.Clean(dir)
	if strings.HasPrefix(dir, "../") || strings.HasSuffix(dir, "/..") || dir == ".." {
		return false
	}
	return true
}

func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}

// func liveListRequest() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		switch r.Method {
// 		case "OPTIONS":
// 			CORSforOptions(&w)
// 			return
// 		case "GET":
// 			// requestBody, err := ioutil.ReadAll(r.Body)
// 			// defer r.Body.Close()
// 			// if err != nil {
// 			// 	responseErrorJSON(w, http.StatusInternalServerError, err.Error())
// 			// 	return
// 			// }

// 			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
// 			client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
// 			if err != nil {
// 				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
// 				return
// 			}
// 			defer client.Disconnect(ctx)

// 			liveCodingCollection := client.Database("liveCoding").Collection("commit")

// 			pipeline := bson.A{
// 				bson.D{{"$group", bson.D{{"_id", "$project_path"}}}},
// 			}
// 			cur, err := liveCodingCollection.Aggregate(ctx, pipeline)
// 			if err != nil {
// 				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
// 				return
// 			}

// 			projectsNameMap := []map[string]string{}

// 			err = cur.All(ctx, &projectsNameMap)
// 			if err != nil {
// 				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
// 				return
// 			}

// 			projectsName := []string{}

// 			for _, value := range projectsNameMap {
// 				projectsName = append(projectsName, value["_id"])
// 			}

// 			// fmt.Println(projectsNameMap)

// 			// fmt.Println(requestBody)

// 			responseJSON(w, http.StatusOK, projectsName)
// 		default:
// 			responseErrorJSON(w, http.StatusMethodNotAllowed, "Sorry, only GET method is supported.")
// 			return
// 		}
// 	}
// }

func liveUploadRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "OPTIONS":
			CORSforOptions(&w)
			return
		case "POST":
			if r.Close == true {
				return
			}

			queryKeys := r.URL.Query()

			queryKey, ok := queryKeys["projectName"]

			if !ok || len(queryKey[0]) < 1 {
				responseErrorJSON(w, http.StatusInternalServerError, "url query 'projectName' is missing")
				return
			}

			projectName := queryKey[0]

			if strings.Contains(projectName, "/") || strings.Contains(projectName, "\\") {
				responseErrorJSON(w, http.StatusInternalServerError, "invalid projectName.")
				return
			}

			// 10MB Limit
			requestBody, err := ioutil.ReadAll(io.LimitReader(r.Body, 10000000))
			defer r.Body.Close()
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			r := bytes.NewReader(requestBody)

			liveLogPath := "./livelog"

			if _, err := os.Stat(liveLogPath); os.IsNotExist(err) {
				err = os.Mkdir(liveLogPath, 0775)
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}
			}

			absliveLogPath, err := filepath.Abs(liveLogPath)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, "upload failed")
				return
			}

			assignProjectNameLength := 20
			assignProjectName := randomText(assignProjectNameLength)

			hostedProjectPath := absliveLogPath + "/" + assignProjectName
			err = os.Mkdir(hostedProjectPath, 0775)
			if err != nil {
				// os.RemoveAll(hostedProjectPath)
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			// fileToWrite, err := os.OpenFile("./compress.tar.gzip", os.O_CREATE|os.O_RDWR, os.FileMode(0644))
			// if err != nil {
			// 	panic(err)
			// }
			// if _, err := io.Copy(fileToWrite, r); err != nil {
			// 	panic(err)
			// }

			prevDir, err := filepath.Abs(".")
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			err = os.Chdir(hostedProjectPath)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			err = untar(r, ".")
			if err != nil {
				// os.RemoveAll(hostedProjectPath)
				responseErrorJSON(w, http.StatusInternalServerError, "upload failed")
				return
			}

			cmd := exec.Command("git", "stash")
			err = cmd.Run()
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			err = os.Chdir(prevDir)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			// hostedPath := hostedProjectPath + "/" + projectName

			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}
			defer client.Disconnect(ctx)

			liveUploadCollection := client.Database("liveCoding").Collection("upload")

			liveUpload := LiveUpload{
				AssignProjectName:   assignProjectName,
				OriginalProjectName: projectName,
				HostedProjectPath:   hostedProjectPath,
			}

			_, err = liveUploadCollection.InsertOne(ctx, liveUpload)
			if err != nil {
				// os.RemoveAll(hostedProjectPath)
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			// fmt.Println(hostedProjectPath)
			gitRepo, err := git.PlainOpen(hostedProjectPath)
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			cIter, err := gitRepo.Log(&git.LogOptions{All: false})
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			var commitObjects []*object.Commit
			err = cIter.ForEach(func(commitObj *object.Commit) error {
				// fmt.Println(commitObj.Hash)
				commitObjects = append(commitObjects, commitObj)
				return nil
			})
			if err != nil {
				responseErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}

			for i, j := 0, len(commitObjects)-1; i < j; i, j = i+1, j-1 {
				commitObjects[i], commitObjects[j] = commitObjects[j], commitObjects[i]
			}

			commitCollection := client.Database("liveCoding").Collection("commit")

			for i := 0; i < len(commitObjects); i++ {
				commitObject := commitObjects[i]
				// fmt.Println(commitObject.Hash)

				commitTime, err := strconv.ParseInt(commitObject.Message, 10, 64)
				if err != nil {
					commitTime = -1
				}
				commitStruct := Commit{
					ProjectPath: hostedProjectPath,
					ProjectName: projectName,
					Hash:        commitObject.Hash.String(),
					Time:        commitTime,
					ID:          i,
				}

				_, err = commitCollection.InsertOne(ctx, commitStruct)
				if err != nil {
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				// fmt.Println(requestBody, 111)
			}

			liveUploadsResponse := LiveUploadsResponse{LiveUploadResponse{URL: "https://localhost:8000/?id=" + assignProjectName}}

			responseJSON(w, http.StatusOK, liveUploadsResponse)
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
				// fmt.Println(err, 111)
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
					// fmt.Println(err, 222)
					responseErrorJSON(w, http.StatusInternalServerError, err.Error())
					return
				}

				err = wt.Checkout(&git.CheckoutOptions{
					Hash: plumbing.NewHash(liveResponse.Hash),
				})
				if err != nil {
					// fmt.Println(err, 333)
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
					// fmt.Println(err, 777)
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
	liveUploadEndpointName := liveEndpointName + "/upload"
	// liveListEndpointName := apiEndpointName + "/liveList"

	http.HandleFunc(liveEndpointName, liveRequest())
	http.HandleFunc(liveUploadEndpointName, liveUploadRequest())
	// http.HandleFunc(liveListEndpointName, liveListRequesst())

	// liveListEndpointName := apiEndpointName + "/liveList"

	schema := config.Schema
	host := config.Host
	port := config.Port
	addr := host + ":" + port
	fmt.Println("LISTEN: ", schema+"://"+addr)
	http.ListenAndServe(addr, nil)
	// 証明書の作成参考: https://ozuma.hatenablog.jp/entry/20130511/1368284304
	/*
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
	*/
}
