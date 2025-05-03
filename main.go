package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/tj/go-naturaldate"
)

const (
	gray   = "\033[38;5;242m"
	yellow = "\033[33m"
	blue   = "\033[1;34m"
	green  = "\033[1;32m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

// Command definitions
type Command struct {
	Aliases     string
	Description string
	Usage       string
	Example     string
}

var commands = map[string]Command{
	"list": {
		Aliases:     "l|list",
		Description: "List all reminders",
		Usage:       "",
		Example:     "",
	},
	"add": {
		Aliases:     "a|add",
		Description: "Add a new reminder",
		Usage:       "<reminder text> [date/time]",
		Example:     "\"Buy groceries tomorrow at 5pm\"",
	},
	"remove": {
		Aliases:     "d|r|del|remove",
		Description: "Remove a reminder by index or text",
		Usage:       "<index|text>",
		Example:     "\"groceries\" or 0",
	},
}

type Reminder struct {
	ID   string
	Text string
}

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
	case "d", "r", "del", "remove":
		if len(os.Args) < 3 {
			fmt.Println("Error: No reminder index or text provided")
			printUsage()
			os.Exit(1)
		}
		target := strings.Join(os.Args[2:], " ")
		removeReminder(target)
	default:
		fmt.Printf("Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func listReminders() {
	cmd := exec.Command("reminders", "show", "Sooner")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error listing reminders: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		index := strings.TrimSpace(parts[0])
		content := strings.TrimSpace(parts[1])

		timeInd := ""
		if idx := strings.LastIndex(content, "("); idx != -1 {
			timeInd = content[idx:]
			content = strings.TrimSpace(content[:idx])
		}

		fmt.Printf("%s[%s%s%s]%s %s", gray, yellow, index, gray, reset, content)

		if timeInd != "" {
			timeText := timeInd[1 : len(timeInd)-1]
			fmt.Printf(" %s(%s%s%s)%s", gray, blue, timeText, gray, reset)
		}
		fmt.Println()
	}
}

// commonDatePatterns returns regex patterns for extracting date references
func commonDatePatterns() []string {
	return []string{
		// Common date patterns
		`tomorrow at \d{1,2}(?::\d{2})? ?(?:am|pm|AM|PM)?`,
		`today at \d{1,2}(?::\d{2})? ?(?:am|pm|AM|PM)?`,
		`next (monday|tuesday|wednesday|thursday|friday|saturday|sunday)`,
		`next week`,
		`next month`,
		`\d{1,2}(?:st|nd|rd|th)? of (january|february|march|april|may|june|july|august|september|october|november|december)`,
		`(january|february|march|april|may|june|july|august|september|october|november|december) \d{1,2}(?:st|nd|rd|th)?`,
		`\d{1,2}\/\d{1,2}(?:\/\d{2,4})?`, // Date formats like MM/DD or MM/DD/YYYY
		`in \d+ (minute|hour|day|week|month|year)s?`,
	}
}

// parseNaturalDate attempts to parse a natural language date expression
func parseNaturalDate(dateStr string) (time.Time, error) {
	// Handle "tomorrow at X" and similar patterns
	now := time.Now()
	return naturaldate.Parse(dateStr, now)
}

// extractDateFromText attempts to find and extract date information from text
// Returns the parsed time, the text with the date portion removed, and whether a date was found
func extractDateFromText(text string) (time.Time, string, bool) {
	// Try using the go-naturaldate library first on the entire text
	parsedTime, err := parseNaturalDate(text)
	if err == nil && parsedTime.After(time.Now()) {
		// If the entire text is interpreted as a date, return it
		return parsedTime, text, true
	}

	// Look for specific date patterns
	patterns := commonDatePatterns()
	cleanText := text

	// Try to find any date expression in the text
	var dateExpression string
	var dateStart, dateEnd int

	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern) // Case insensitive
		loc := re.FindStringIndex(text)
		if loc != nil {
			dateStart = loc[0]
			dateEnd = loc[1]
			dateExpression = text[dateStart:dateEnd]
			break
		}
	}

	// If we found a date expression, try to parse it
	if dateExpression != "" {
		parsedTime, err = parseNaturalDate(dateExpression)
		if err == nil && parsedTime.After(time.Now()) {
			// Remove the date expression from the text
			cleanText = text[:dateStart] + text[dateEnd:]
			cleanText = strings.TrimSpace(cleanText)
			return parsedTime, cleanText, true
		}
	}

	// No valid date found
	return time.Time{}, text, false
}

func addReminder(text string) {
	parsedTime, cleanText, hasDate := extractDateFromText(text)

	var reminderCmd *exec.Cmd

	if !hasDate || parsedTime.IsZero() {
		// No date found, add reminder without a date
		reminderCmd = exec.Command("reminders", "add", "Sooner", text)
		cleanText = text // Use original text
	} else {
		// Format the date for the reminders command
		dateTime := parsedTime.Format("2006-01-02 15:04:05")
		reminderCmd = exec.Command("reminders", "add", "Sooner", cleanText, "-d", dateTime)
	}

	reminderCmd.Stderr = os.Stderr

	if err := reminderCmd.Run(); err != nil {
		fmt.Printf("Error adding reminder: %v\n", err)
		os.Exit(1)
	} else {
		fmt.Printf("Added %s'%s'%s", gray, green+cleanText+gray, reset)
		if hasDate && !parsedTime.IsZero() {
			fmt.Printf(" %s(parsed as: %s%s%s)%s",
				gray,
				blue,
				parsedTime.Format("Mon Jan 2 15:04:05"),
				gray,
				reset)
		}
		fmt.Println()
	}
}

func removeReminder(target string) {
	reminders, err := getReminders()
	if err != nil {
		fmt.Printf("Error getting reminders: %v\n", err)
		os.Exit(1)
	}

	if len(reminders) == 0 {
		fmt.Println("No reminders found")
		return
	}

	if index, err := strconv.Atoi(target); err == nil {
		if index >= 0 && index < len(reminders) {
			removeReminderByID(reminders[index].ID)
			return
		}
		fmt.Printf("Invalid index: %d. Valid range is 0-%d\n", index, len(reminders)-1)
		return
	}

	bestMatch := ""
	bestScore := 0.0
	bestIndex := -1

	for i, reminder := range reminders {
		distance := fuzzy.LevenshteinDistance(target, reminder.Text)
		maxLen := float64(max(len(target), len(reminder.Text)))
		normalizedScore := 1.0 - (float64(distance) / maxLen)

		if normalizedScore > bestScore {
			bestScore = normalizedScore
			bestMatch = reminder.Text
			bestIndex = i
		}
	}

	if bestScore < 0.50 {
		fmt.Printf("No reminder found matching '%s' with at least 50%% similarity\n", target)
		return
	}

	fmt.Printf("Removing reminder: %s (%.0f%% match)\n", bestMatch, bestScore*100)
	removeReminderByID(reminders[bestIndex].ID)
}

func getReminders() ([]Reminder, error) {
	cmd := exec.Command("reminders", "show", "Sooner")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var reminders []Reminder
	scanner := bufio.NewScanner(&out)
	re := regexp.MustCompile(`^(\d+): (.+?)( \(in .+\))?$`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 3 {
			reminders = append(reminders, Reminder{
				ID:   matches[1],
				Text: strings.TrimSuffix(matches[2], " "),
			})
		}
	}

	return reminders, nil
}

func removeReminderByID(id string) {
	cmd := exec.Command("reminders", "delete", "Sooner", id)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error removing reminder: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	for _, cmd := range commands {
		usage := cmd.Usage
		if usage != "" {
			usage = " " + usage
		}
		fmt.Printf("  qr %s%s\t%s\n", cmd.Aliases, usage, cmd.Description)
	}

	fmt.Println("\nExamples:")
	for _, cmd := range commands {
		if cmd.Example != "" {
			alias := strings.Split(cmd.Aliases, "|")[0]
			fmt.Printf("  qr %s %s\n", alias, cmd.Example)
		}
	}
	fmt.Println()
}

// Go 1.21+ has built-in max, but providing it here for compatibility
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
