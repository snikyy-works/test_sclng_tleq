package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/Scalingo/go-handlers"
	"github.com/Scalingo/go-utils/logger"
)

// RepositoryFromAPI struct according to the result of GithubAPI
type RepositoryFromAPI struct {
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	RepoName     string `json:"name"`
	LanguagesURL string `json:"languages_url"`
}

// LanguageDetail holds the byte count for each programming language.
type LanguageDetail struct {
	Bytes int `json:"bytes"`
}

// Language is a map where the key is the language name and the value is LanguageDetail.
type Language map[string]LanguageDetail

// Repository struct with data formatted
type Repository struct {
	FullName  string   `json:"full_name"`
	Owner     string   `json:"owner"`
	RepoName  string   `json:"repository"`
	Languages Language `json:"languages"`
}

// Struct for the final response to match the JSON format of the exercise
type RepositoriesResponse struct {
	Repositories []Repository `json:"repositories"`
}

// Github API URL for the 100 last public repositories
const githubAPIURL = "https://api.github.com/repositories?per_page=100&page=1"

// Github Token to avoid rate limits defined by default with GithubAPI
var githubToken string

func main() {
	log := logger.Default()
	log.Info("Initializing app")
	cfg, err := newConfig()
	if err != nil {
		log.WithError(err).Error("Fail to initialize configuration")
		os.Exit(1)
	}

	// Retrieve GITHUB_TOKEN var to authenticate to the Github API
	githubToken = os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.WithError(fmt.Errorf("GITHUB_TOKEN environment variable not set in .env file")).Error("GITHUB_TOKEN env var not set")
		os.Exit(1)
	}

	log.Info("Initializing routes")
	router := handlers.NewRouter(log)
	router.HandleFunc("/ping", pongHandler)
	// Initialize web server and configure the following routes:
	// GET /repos - Get all repositories
	router.HandleFunc("/repos", repositoriesHandler)
	// GET /repos/lang/{lang} - Get all repositories containing {lang} as programming languages
	router.HandleFunc("/repos/lang/{lang}", repositoriesHandler)
	// GET /repos/owner/{owner} - Get all repositories owned by {owner}
	router.HandleFunc("/repos/owner/{owner}", repositoriesHandler)

	log = log.WithField("port", cfg.Port)
	log.Info("Listening...")
	err = http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), router)
	if err != nil {
		log.WithError(err).Error("Fail to listen to the given port")
		os.Exit(2)
	}
}

func pongHandler(w http.ResponseWriter, r *http.Request, _ map[string]string) error {
	log := logger.Get(r.Context())
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(map[string]string{"status": "pong"})
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}
	return nil
}

// repositoriesHandler handles request to GithubAPI and formats data for a specific request
func repositoriesHandler(w http.ResponseWriter, r *http.Request, params map[string]string) error {

	log := logger.Get(r.Context())
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Retrieve filter if exists
	var filterType, filterValue string
	for fType, fValue := range params {
		filterType = fType
		filterValue = fValue
	}

	// Create the GET Request to GithubAPI to retrieve repositories
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		log.WithError(err).Error("Fail to create GET Request to fetch repositories")
	}
	// Set Auth with the github token defined in .env file
	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithError(err).Error("Fail to make GET Request to fetch repositories")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithError(fmt.Errorf(resp.Status)).Error("HTTP Status non-OK after fetching repositories")
	}

	// Parse response
	var repositories []RepositoryFromAPI
	if err := json.NewDecoder(resp.Body).Decode(&repositories); err != nil {
		log.Fatalf("Error decoding response from GithubAPI: %v", err)
	}

	// Setup to fetch languages data concurrently
	var wg sync.WaitGroup
	reposResponse := make([]Repository, 0, len(repositories))
	resultsCh := make(chan Repository) // Channel to collect results
	errCh := make(chan error)          // Channel to collect errors

	for _, repo := range repositories {
		wg.Add(1)
		go func(repo RepositoryFromAPI) {
			defer wg.Done()

			// Temporary map to hold raw languages data
			languages := make(map[string]int)
			languagesURL := repo.LanguagesURL

			// Create the GET Request to GithubAPI to retrieve languages data
			req, err := http.NewRequest("GET", languagesURL, nil)
			if err != nil {
				errCh <- fmt.Errorf("failed to create request for languages for %s: %w", repo.FullName, err)
				return
			}

			// Set Auth with the github token defined in .env file
			req.Header.Set("Authorization", "token "+githubToken)

			resp, err := client.Do(req)
			if err != nil {
				errCh <- fmt.Errorf("error fetching languages for %s: %w", repo.FullName, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("error fetching languages for %s: %s", repo.FullName, resp.Status)
				return
			}

			if err := json.NewDecoder(resp.Body).Decode(&languages); err != nil {
				errCh <- fmt.Errorf("error decoding languages for %s: %w", repo.FullName, err)
				return
			}

			// Convert the raw map to the desired structure
			languagesDetail := make(Language)
			for lang, bytes := range languages {
				languagesDetail[lang] = LanguageDetail{Bytes: bytes} // Wrap byte count in LanguageDetail
			}

			// Create a repository object
			repoResponse := Repository{
				FullName:  repo.FullName,
				Owner:     repo.Owner.Login,
				RepoName:  repo.RepoName,
				Languages: languagesDetail,
			}

			// Send the result to the channel
			resultsCh <- repoResponse
		}(repo)
	}

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(resultsCh)
		close(errCh)
	}()

	// Collect results and handle errors
	for {
		select {
		case repo, ok := <-resultsCh:
			if !ok {
				// If results channel is closed, set it to nil
				resultsCh = nil
			} else {
				reposResponse = append(reposResponse, repo)
			}
		case err, ok := <-errCh:
			if !ok {
				// If error channel is closed, set it to nil
				errCh = nil
			} else {
				log.WithError(err).Error("Error occurred during language fetching")
				return err // Return the first error encountered
			}
		}
		// Exit the loop when both channels are closed
		if resultsCh == nil && errCh == nil {
			break
		}
	}

	// If a filter was provided, then filter the results
	if filterType != "" {
		reposResponse = filterByType(filterType, filterValue, reposResponse)
	}

	// Create final response format to match the JSON format of the exercise
	response := RepositoriesResponse{
		Repositories: reposResponse,
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.WithError(err).Error("Fail to encode JSON")
	}

	return nil
}

// filterByType filters repositories by language or owner
// You can add more filter types here by adding new case in switch statements
func filterByType(filterType, filterValue string, repos []Repository) []Repository {
	var filteredRepos []Repository
	switch filterType {
	case "lang":
		for _, repo := range repos {
			if _, exists := repo.Languages[filterValue]; exists {
				filteredRepos = append(filteredRepos, repo)
			}
		}
	case "owner":
		for _, repo := range repos {
			if repo.Owner == filterValue {
				filteredRepos = append(filteredRepos, repo)
			}
		}
	default:
		return repos
	}
	return filteredRepos
}
