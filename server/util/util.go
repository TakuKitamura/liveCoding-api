package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Commands struct {
	Content string `json:"content" bson:"content"`
}

func GetCommands(code string, lang string) (error, Commands) {
	var commands Commands
	var commentMark string
	if lang == "python" || lang == "bash" {
		commentMark = "#"
	} else if lang == "plaintext" || lang == "javascript" || lang == "go" || lang == "c" {
		commentMark = "//"
	} else if lang == "html" {
		commentMark = "<!--"
	} else {
		errMsg := "unsupport lang"
		return errors.New(errMsg), commands
	}

	commandMark := commentMark + "@"
	splitCode := strings.Split(code, "\n")
	for i := 0; i < len(splitCode); i++ {
		line := splitCode[i]
		trimSpace := strings.TrimLeft(line, " 	")
		if len(trimSpace) < len(commandMark) {
			return nil, commands
		}
		mayBeCommandMark := trimSpace[0:len(commandMark)]
		if mayBeCommandMark != commandMark {
			return nil, commands
		}
		commandMark := mayBeCommandMark
		coommandsJSON := "{" + trimSpace[len(commandMark):len(trimSpace)] + "}"
		fmt.Println(coommandsJSON, 999)
		err := json.Unmarshal([]byte(coommandsJSON), &commands)
		if err != nil {
			errMsg := "command is inavlid"
			return errors.New(errMsg), commands
		}
	}
	return nil, commands
}
