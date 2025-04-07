package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "l", "list":
		listReminders()
	case "a", "add":
		if len(os.Args) < 3 {
			fmt.Println("Error: No reminder text provided")
			printUsage()
			os.Exit(1)
		}
		reminderText := strings.Join(os.Args[2:], " ")
		addReminder(reminderText)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func listReminders() {
	cmd := exec.Command("reminders", "show", "Sooner")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error listing reminders: %v\n", err)
		os.Exit(1)
	}
}

func addReminder(text string) {
	// Parse date and time from the text
	w := when.New(nil)
	w.Add(en.All...)
	w.Add(common.All...)

	now := time.Now()
	result, err := w.Parse(text, now)
	
	var reminderCmd *exec.Cmd
	
	if err != nil || result == nil {
		// No date/time found, add as a regular reminder
		reminderCmd = exec.Command("reminders", "add", "Sooner", text)
	} else {
		// Extract the date/time and the actual reminder text
		dateTime := result.Time.Format("2006-01-02 15:04:05")
		
		// Remove the date/time part from the reminder text
		cleanText := text[:result.Index] + text[result.Index+len(result.Text):]
		cleanText = strings.TrimSpace(cleanText)
		
		// If the text is empty after removing the date, use a generic message
		if cleanText == "" {
			cleanText = "Reminder"
		}
		
		// Add reminder with the specified date and time
		reminderCmd = exec.Command("reminders", "add", "Sooner", cleanText, "-d", dateTime)
	}
	
	reminderCmd.Stdout = os.Stdout
	reminderCmd.Stderr = os.Stderr
	
	if err := reminderCmd.Run(); err != nil {
		fmt.Printf("Error adding reminder: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  qr l|list                                List all reminders")
	fmt.Println("  qr a|add <reminder text> [date/time]     Add a new reminder")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  qr a Buy groceries")
	fmt.Println("  qr a Call mom tomorrow at 5pm")
	fmt.Println("  qr a Submit report next Friday")
}
