package main

import (
	"liveCoding/server/util"
	"testing"
)

func TestExampleSuccess(t *testing.T) {
	err, commands := util.GetCommands("#@ \"content\": \"見出し1\"", "python")
	if err != nil {
		t.Fatalf("failed test1 %s", err.Error())
	}

	if commands.Content != "見出し1" {
		t.Fatalf("failed test2")
	}

	err, commands = util.GetCommands("//@ \"content\": \"見出し1\"", "plaintext")
	if err != nil {
		t.Fatalf("failed test3 %s", err.Error())
	}

	if commands.Content != "見出し1" {
		t.Fatalf("failed test4")
	}
}
