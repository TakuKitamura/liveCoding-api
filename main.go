package main

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/src-d/go-git.v4"
	. "gopkg.in/src-d/go-git.v4/_examples"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

type Commit struct {
	ProjectName string `bson:"project_name"`
	Hash        string `bson:"hash"`
	Time        int64  `bson:"time"`
	ID          int    `bson:"id"`
	// Files       map[string]string `bson:"files"`
}

type Commits []Commit

func main() {
	directory := os.Args[1]

	var r *git.Repository
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		r, err = git.PlainInit(directory, false)
		CheckIfError(err)
	} else {
		r, err = git.PlainOpen(directory)
		CheckIfError(err)
	}

	w, err := r.Worktree()
	CheckIfError(err)

	i := 0
	for {
		time.Sleep(time.Millisecond * 100)

		// TODO: 対応があれば置き換え｡
		// _, err := w.Add(".")
		// CheckIfError(err)

		w.Checkout(&git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName("master"),
		})
		// CheckIfError(err)

		cmd := exec.Command("git", "add", ".")
		cmd.Dir = w.Filesystem.Root()
		err = cmd.Run()
		CheckIfError(err)

		status, err := w.Status()
		CheckIfError(err)
		if len(status) != 0 {
			commit, err := w.Commit(strconv.FormatInt(time.Now().UnixNano(), 10), &git.CommitOptions{
				Author: &object.Signature{
					When: time.Now(),
				},
			})
			CheckIfError(err)
			obj, err := r.CommitObject(commit)
			CheckIfError(err)

			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
			CheckIfError(err)
			defer client.Disconnect(ctx)

			// files := map[string]string{}
			// err = filepath.Walk(directory,
			// 	func(path string, info os.FileInfo, err error) error {
			// 		CheckIfError(err)
			// 		if path != directory {
			// 			if strings.HasPrefix(path, directory+".git") == false {
			// 				file, err := os.Stat(path)
			// 				CheckIfError(err)
			// 				if file.Mode().IsRegular() {
			// 					bytes, err := ioutil.ReadFile(path)
			// 					CheckIfError(err)
			// 					files[path] = string(bytes)
			// 					i += 1
			// 				}
			// 			}
			// 		}
			// 		return nil
			// 	})
			// CheckIfError(err)

			commitTime, err := strconv.ParseInt(obj.Message, 10, 64)
			CheckIfError(err)

			commitStruct := Commit{
				ProjectName: "test",
				Hash:        obj.Hash.String(),
				Time:        commitTime,
				ID:          i,
				// Files:       files,
			}
			commit_collection := client.Database("liveCoding").Collection("commit")
			_, err = commit_collection.InsertOne(ctx, commitStruct)
			CheckIfError(err)

			i++

		}
	}
}
