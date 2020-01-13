package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func remove(strings []string, search string) []string {
	result := []string{}
	for _, v := range strings {
		if v != search {
			result = append(result, v)
		}
	}
	return result
}

func main() {
	buffer := &bytes.Buffer{}
	writer := buffer
	projectPath := "/Users/kitamurataku/work/go/src/liveCoding/test"
	err := os.Chdir(projectPath)
	if err != nil {
		fmt.Print(err.Error())
		return
	}

	for {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("$ ")
		scanner.Scan()
		line := scanner.Text()

		cmdSplit := strings.Split(line, " ")
		cmdSplit = remove(cmdSplit, "")

		if len(cmdSplit) > 0 {
			firstCommandName := cmdSplit[0]
			if firstCommandName == "cd" {
				if len(cmdSplit) > 1 {
					err := os.Chdir(cmdSplit[1])
					if err != nil {
						fmt.Print(err.Error())
						continue
					}
					continue
				} else {
					home, err := os.UserHomeDir()
					if err != nil {
						fmt.Print(err.Error())
						continue
					}
					os.Chdir(home)
					continue
				}
			}
		}

		cmd := exec.Command("bash", "-c", line)
		cmd.Stdout = writer
		cmd.Stderr = writer

		cmd.Run()
		out := buffer.String()
		fmt.Print(out)
		buffer.Reset()

		file, err := os.OpenFile(projectPath+"/cui.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}
		// defer
		fmt.Fprintln(file, "$ "+line)
		fmt.Fprint(file, out)
		file.Close()
	}
}
