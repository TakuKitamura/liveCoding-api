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
	projectPath string `bson:"project_path"`
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

		time.Sleep(time.Second * 1)

		// TODO: 対応があれば置き換え｡
		// _, err := w.Add(".")
		// CheckIfError(err)

		// CheckIfError(err)

		status, err := w.Status()
		if err != nil {
			continue
		}

		if len(status) != 0 {
			cmd := exec.Command("git", "add", ".")
			cmd.Dir = w.Filesystem.Root()
			err = cmd.Run()
			if err != nil {
				continue
			}

			commit, err := w.Commit(strconv.FormatInt(time.Now().UnixNano(), 10), &git.CommitOptions{
				Author: &object.Signature{
					When: time.Now(),
				},
			})
			if err != nil {
				continue
			}

			w.Checkout(&git.CheckoutOptions{
				Branch: plumbing.NewBranchReferenceName("master"),
			})

			obj, err := r.CommitObject(commit)
			if err != nil {
				continue
			}

			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
			if err != nil {
				continue
			}
			defer client.Disconnect(ctx)

			commitTime, err := strconv.ParseInt(obj.Message, 10, 64)
			if err != nil {
				continue
			}

			commitStruct := Commit{
				ProjectName: "test",
				Hash:        obj.Hash.String(),
				Time:        commitTime,
				ID:          i,
				// Files:       files,
			}
			commit_collection := client.Database("liveCoding").Collection("commit")
			_, err = commit_collection.InsertOne(ctx, commitStruct)
			if err != nil {
				continue
			}

			i++

		}
	}
}
