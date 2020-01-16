package util

import (
	"encoding/json"
	"errors"
	"strings"
)

type Commands struct {
	Content string `json:"content" bson:"content"`
}

func GetCommands(code string, lang string, basename string) (error, Commands) {
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
	// fmt.Println(splitCode)
	for i := 0; i < len(splitCode); i++ {
		line := splitCode[i]
		// fmt.Println(line)
		var trimSpace string
		if basename == ".cui.log" {
			trimSpace = strings.TrimLeft(line, " 	$")
		} else {
			trimSpace = strings.TrimLeft(line, " 	")
		}

		// fmt.Println(line, trimSpace, 7777777777)
		if len(trimSpace) < len(commandMark) {
			continue
			// return nil, commands
		}
		mayBeCommandMark := trimSpace[0:len(commandMark)]
		// fmt.Println(mayBeCommandMark, "AAA", commandMark, "BBB", mayBeCommandMark != commandMark, "CCC", line)
		if mayBeCommandMark != commandMark {
			continue
			// return nil, commands
		}
		// fmt.Println(code)
		commandMark := mayBeCommandMark
		coommandsJSON := "{" + trimSpace[len(commandMark):len(trimSpace)] + "}"
		// fmt.Println(coommandsJSON, trimSpace, 999)
		err := json.Unmarshal([]byte(coommandsJSON), &commands)
		if err != nil {
			// errMsg := "command is inavlid"
			// return errors.New(errMsg), commands
			continue
		}
	}
	return nil, commands
}
