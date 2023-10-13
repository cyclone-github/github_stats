package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

/*
tool to retrieve user stats from github
coded by cyclone
v0.2.11; initial github release
v2023-10-13.1645; fixed watchers; added cache and ratelimiting support
*/

// structs for config.json file
type Owner struct {
	Login string `json:"login"`
}
type Repository struct {
	Name           string    `json:"name"`
	Stars          int       `json:"stargazers_count"`
	Watchers       int       `json:"watchers_count"`
	Forks          int       `json:"forks_count"`
	OpenIssues     int       `json:"open_issues_count"`
	SubscribersURL string    `json:"subscribers_url"`
	Owner          Owner     `json:"owner"`
	LastFetched    time.Time `json:"last_fetched"`
}

// local json config struct
type Config struct {
	Usernames []string              `json:"usernames"`
	ReposData map[string]Repository `json:"repos_data"`
}

// x-compatible clear screen func
func clearScreen() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd = exec.Command("clear")
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		fmt.Fprintf(os.Stderr, "Unsupported OS: %s\n", runtime.GOOS)
		os.Exit(1)
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// user select func
func selectUsername(config *Config, configFile *os.File) string {
	scanner := bufio.NewScanner(os.Stdin)

	if len(config.Usernames) == 0 {
		fmt.Fprint(os.Stderr, "Enter a GitHub username: ")
		scanner.Scan()
		return scanner.Text()
	}

	for {
		fmt.Fprintln(os.Stderr, "\nPlease Select User:")
		for i, user := range config.Usernames {
			fmt.Fprintf(os.Stderr, "%d. %s\n", i+1, user)
		}
		fmt.Fprintf(os.Stderr, "N. New User\n")
		fmt.Fprintf(os.Stderr, "R. Remove User\n")
		fmt.Fprintf(os.Stderr, "Q. Quit\n")

		fmt.Fprint(os.Stderr, "Enter your choice: ")
		scanner.Scan()
		input := scanner.Text()

		if input == "n" || input == "N" {
			fmt.Fprint(os.Stderr, "Enter a GitHub username: ")
			scanner.Scan()
			return scanner.Text()
		} else if input == "r" || input == "R" {
			removeUser(config, configFile)
			continue
		} else if input == "q" || input == "Q" {
			os.Exit(0)
			continue
		} else {
			choice, err := strconv.Atoi(input)
			if err == nil && choice > 0 && choice <= len(config.Usernames) {
				return config.Usernames[choice-1]
			} else {
				fmt.Fprintln(os.Stderr, "Invalid choice, please try again.")
				continue
			}
		}
	}
}

// add user func
func addUsername(config *Config, username string) {
	for _, user := range config.Usernames {
		if user == username {
			return
		}
	}
	config.Usernames = append(config.Usernames, username)
}

// remove user func
func removeUser(config *Config, configFile *os.File) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Fprintln(os.Stderr, "\nSelect User to Remove:")
		for i, user := range config.Usernames {
			fmt.Fprintf(os.Stderr, "%d. %s\n", i+1, user)
		}
		fmt.Fprintf(os.Stderr, "%d. Go Back\n", len(config.Usernames)+1)

		fmt.Fprint(os.Stderr, "Enter your choice: ")
		scanner.Scan()
		input := scanner.Text()

		if input == strconv.Itoa(len(config.Usernames)+1) {
			return
		}

		choice, err := strconv.Atoi(input)
		if err == nil && choice > 0 && choice <= len(config.Usernames) {
			usernameToRemove := config.Usernames[choice-1]
			config.Usernames = append(config.Usernames[:choice-1], config.Usernames[choice:]...)

			for repoName, repoData := range config.ReposData {
				if repoData.Owner.Login == usernameToRemove {
					delete(config.ReposData, repoName)
				}
			}
			configFile.Seek(0, 0)
			configFile.Truncate(0)
			err = json.NewEncoder(configFile).Encode(&config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding config file: %v\n", err)
			}

			saveConfig(config, configFile) // save updated config after removing user
			return
		}
		fmt.Fprintln(os.Stderr, "Invalid choice, please try again.")
	}
}

// save config func
func saveConfig(config *Config, configFile *os.File) {
	configFile.Seek(0, 0)
	configFile.Truncate(0)
	err := json.NewEncoder(configFile).Encode(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding config file: %v\n", err)
	}
}

// get number of subscribers ("Watchers") for a repository
func getSubscribersCount(subscribersURL string) int {
	resp, err := http.Get(subscribersURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching subscribers: %v\n", err)
		return -1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading subscribers response: %v\n", err)
		return -1
	}
	var subscribers []interface{}
	err = json.Unmarshal(body, &subscribers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding subscribers response: %v\n", err)
		return -1
	}
	return len(subscribers)
}

func getRepositories(username string, config *Config) ([]Repository, error) {
	var allRepos []Repository
	cacheFile := username + "_cache.json"

	// check cache
	fileInfo, err := os.Stat(cacheFile)
	if err == nil {
		cacheTime := fileInfo.ModTime()
		if time.Since(cacheTime).Minutes() < 10 { // nth minutes cache time
			file, _ := os.ReadFile(cacheFile)
			json.Unmarshal(file, &allRepos)
			return allRepos, nil
		}
	}

	// Fetch data from API
	perPage := 100
	page := 1

	for {
		url := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=%d&page=%d", username, perPage, page)
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("error fetching repositories: %v", err)
		}
		defer resp.Body.Close()

		// check rate limits
		remaining, resetTime := getRateLimits(resp.Header)
		if remaining == 0 {
			return nil, fmt.Errorf("rate limit exceeded, resets at %s", resetTime)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error: HTTP %d", resp.StatusCode)
		}

		var repos []Repository
		err = json.NewDecoder(resp.Body).Decode(&repos)
		if err != nil {
			return nil, fmt.Errorf("error decoding response: %v", err)
		}

		if len(repos) == 0 {
			break
		}

		allRepos = append(allRepos, repos...)
		page++
	}

	for i := range allRepos {
		allRepos[i].Watchers = getSubscribersCount(allRepos[i].SubscribersURL)
	}

	// save to cache
	file, _ := json.MarshalIndent(allRepos, "", " ")
	_ = os.WriteFile(cacheFile, file, 0644)

	return allRepos, nil
}

// extract GitHub rate limit headers
func getRateLimits(headers http.Header) (int, time.Time) {
	remaining := 0
	if remainingStr := headers.Get("X-RateLimit-Remaining"); remainingStr != "" {
		remaining, _ = strconv.Atoi(remainingStr)
	}

	resetTime := time.Time{}
	if resetStr := headers.Get("X-RateLimit-Reset"); resetStr != "" {
		if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			resetTime = time.Unix(resetUnix, 0)
		}
	}

	return remaining, resetTime
}

// print repositories func
func printRepository(repo Repository, prevData Repository) {
	starsDiff := repo.Stars - prevData.Stars
	watchersDiff := repo.Watchers - prevData.Watchers
	forksDiff := repo.Forks - prevData.Forks
	issuesDiff := repo.OpenIssues - prevData.OpenIssues

	starChange := ""
	if starsDiff != 0 {
		starChange = fmt.Sprintf("%+d", starsDiff)
	}

	watcherChange := ""
	if watchersDiff != 0 {
		watcherChange = fmt.Sprintf("%+d", watchersDiff)
	}

	forkChange := ""
	if forksDiff != 0 {
		forkChange = fmt.Sprintf("%+d", forksDiff)
	}

	issuesChange := ""
	if issuesDiff != 0 {
		issuesChange = fmt.Sprintf("%+d", issuesDiff)
	}

	repoName := fmt.Sprintf("%.30s", repo.Name)
	starsStr := fmt.Sprintf("%5d %4s", repo.Stars, starChange)
	watchersStr := fmt.Sprintf("%5d %4s", repo.Watchers, watcherChange)
	forksStr := fmt.Sprintf("%5d %4s", repo.Forks, forkChange)
	openIssuesStr := fmt.Sprintf("%5d %4s", repo.OpenIssues, issuesChange)
	rowData := fmt.Sprintf("%-30s\t | %-5s\t | %-5s\t | %-5s\t | %-5s\t", repoName, starsStr, watchersStr, forksStr, openIssuesStr)

	fmt.Printf("%-72s\n", rowData)
}

// main func
func main() {
	cycloneFlag := flag.Bool("cyclone", false, "")
	versionFlag := flag.Bool("version", false, "Version number")
	helpFlag := flag.Bool("help", false, "Program usage instructions")
	flag.Parse()

	clearScreen()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	if *cycloneFlag {
		codedBy := "Q29kZWQgYnkgY3ljbG9uZSA7KQo="
		codedByDecoded, _ := base64.StdEncoding.DecodeString(codedBy)
		fmt.Fprintln(os.Stderr, string(codedByDecoded))
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Fprintln(os.Stderr, "Cyclone GitHub Stats v2023-10-13.1645")
		os.Exit(0)
	}

	configFile, err := os.OpenFile("config.json", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening config file: %v\n", err)
		os.Exit(1)
	}
	defer configFile.Close()

	var config Config

	fileInfo, err := configFile.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting file info: %v\n", err)
		os.Exit(1)
	}

	if fileInfo.Size() > 0 {
		err = json.NewDecoder(configFile).Decode(&config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding config file: %v\n", err)
			os.Exit(1)
		}
	} else {
		config = Config{
			Usernames: []string{},
			ReposData: make(map[string]Repository),
		}
	}

	for {
		fmt.Fprintln(os.Stderr, " ------------------------ ")
		fmt.Fprintln(os.Stderr, "| Cyclone's GitHub Stats |")
		fmt.Fprintln(os.Stderr, " ------------------------ ")
		fmt.Fprintln(os.Stderr)

		username := selectUsername(&config, configFile)

		fmt.Fprintln(os.Stderr, "Fetching repositories...")

		repos, err := getRepositories(username, &config)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		sort.Slice(repos, func(i, j int) bool {
			return strings.ToLower(repos[i].Name) < strings.ToLower(repos[j].Name)
		})

		fmt.Println("")
		fmt.Printf("%-32s | %-13s | %-13s | %-13s | %-13s\n", "Repository", "    Stars", "   Watchers", "    Forks", " Open Issues")
		fmt.Println("")

		for _, repo := range repos {
			printRepository(repo, config.ReposData[repo.Name])
		}

		addUsername(&config, username)
		for _, repo := range repos {
			config.ReposData[repo.Name] = repo
		}

		// Save to config
		configFile.Seek(0, 0)
		configFile.Truncate(0)
		err = json.NewEncoder(configFile).Encode(&config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding config file: %v\n", err)
		}
		fmt.Println()
		os.Exit(0)
	}
}

// end code
