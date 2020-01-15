package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

func writeCommandInput(input string, projectPath string, liveStart bool) {
	if liveStart == false {
		return
	}
	file, err := os.OpenFile(projectPath+"/cui.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	// defer
	fmt.Fprintln(file, "$ "+input)
	file.Close()
}

func writeCommandOut(out string, projectPath string, liveStart bool) {
	fmt.Print(out)
	if liveStart == false {
		return
	}
	file, err := os.OpenFile(projectPath+"/cui.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	// defer
	fmt.Fprint(file, out)
	file.Close()
}

func liveCommandUsage(projectPath string) {
	out := "usage: live [start, stop]\n"
	writeCommandOut(out, projectPath, false)
}

func main() {
	projectPath := ""
	buffer := &bytes.Buffer{}
	writer := buffer
	liveStart := false

	// err := os.Chdir(projectPath)
	// if err != nil {
	// 	fmt.Print(err.Error())
	// 	return
	// }

	for {
		pwd, err := os.Getwd()
		if err != nil {
			writeCommandOut(err.Error()+"\n", projectPath, liveStart)
			continue
		}

		home, err := os.UserHomeDir()
		if err != nil {
			writeCommandOut(err.Error()+"\n", projectPath, liveStart)
			continue
		}

		var liveStatus string
		if liveStart {
			liveStatus = "recording"
		} else {
			liveStatus = "stopped"
		}

		currentPath := strings.Replace(pwd, home, "~", 1)
		// fmt.Println(p)
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Printf("\x1b[34m%s\x1b[0m \x1b[31m(%s)\x1b[0m %s ", currentPath, liveStatus, "$")
		scanner.Scan()
		line := scanner.Text()

		// file, err := os.OpenFile(projectPath+"/cui.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// // defer
		// fmt.Fprintln(file, "$ "+line)

		cmdSplit := strings.Split(line, " ")
		cmdSplit = remove(cmdSplit, "")

		if len(cmdSplit) > 0 {
			firstCommandName := cmdSplit[0]
			if firstCommandName == "cd" {
				writeCommandInput(line, projectPath, liveStart)
				if len(cmdSplit) > 1 {
					err := os.Chdir(cmdSplit[1])
					if err != nil {
						writeCommandOut(err.Error()+"\n", projectPath, liveStart)
						continue
					}
					continue
				} else {
					home, err := os.UserHomeDir()
					if err != nil {
						writeCommandOut(err.Error()+"\n", projectPath, liveStart)
						continue
					}
					err = os.Chdir(home)
					if err != nil {
						writeCommandOut(err.Error()+"\n", projectPath, liveStart)
						continue
					}
					continue
				}
			} else if firstCommandName == "live" {
				if len(cmdSplit) == 2 {
					secondCommandValue := cmdSplit[1]
					if secondCommandValue == "stop" {
						liveStart = false
						continue
					} else if secondCommandValue == "status" {
						if liveStart {
							writeCommandOut("live is started.\n", projectPath, false)
						} else {
							writeCommandOut("live is stopped.\n", projectPath, false)
						}
						continue
					} else {
						liveCommandUsage(projectPath)
						continue
					}
				} else if len(cmdSplit) == 3 {
					secondCommandValue := cmdSplit[1]
					thirdCommandValue := cmdSplit[2]
					if secondCommandValue == "start" {
						if liveStart {
							writeCommandOut("live is already started.\n", projectPath, liveStart)
							continue
						}
						absPath, err := filepath.Abs(thirdCommandValue)
						if err != nil {
							writeCommandOut(err.Error()+"\n", projectPath, liveStart)
							continue
						}
						if _, err := os.Stat(absPath); !os.IsNotExist(err) {
							writeCommandOut("can't live in the path.\n", projectPath, liveStart)
							continue
						}

						if err := os.Mkdir(absPath, 0751); err != nil {
							writeCommandOut(err.Error()+"\n", projectPath, liveStart)
							continue
						}

						projectPath = absPath

						// fmt.Println(absPath)
						liveStart = true
						continue
					} else if secondCommandValue == "resume" {
						if liveStart {
							writeCommandOut("live is already started.\n", projectPath, false)
							continue
						}
						absPath, err := filepath.Abs(thirdCommandValue)
						if err != nil {
							writeCommandOut(err.Error()+"\n", projectPath, liveStart)
							continue
						}
						if _, err := os.Stat(absPath); os.IsNotExist(err) {
							writeCommandOut("File doesn't exists\n", projectPath, liveStart)
							continue
						}

						projectPath = absPath
						// fmt.Println(absPath)
						liveStart = true
					} else {
						liveCommandUsage(projectPath)
						continue
					}
				} else {
					liveCommandUsage(projectPath)
					continue
				}
			} else {
				writeCommandInput(line, projectPath, liveStart)
				cmd := exec.Command("bash", "-c", line)
				cmd.Stdout = writer
				cmd.Stderr = writer

				cmd.Run()
				out := buffer.String()
				// fmt.Print(out)
				buffer.Reset()

				writeCommandOut(out, projectPath, liveStart)
			}
		}

	}
}
