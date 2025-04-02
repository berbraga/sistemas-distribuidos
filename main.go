package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Estrutura para armazenar os dados do filme
type Movie struct {
	Title    string   `json:"Title"`
	Year     string   `json:"Year"`
	Rated    string   `json:"Rated"`
	Runtime  string   `json:"Runtime"`
	Rating   string   `json:"Rating"`
	Comments []string `json:"Comments"`
}

// Substitua pela sua chave de API do OMDb e TMDb
const apiKeyOMDB = "db87d64c"
const apiKeyTMDB = "10642f172ea2a7371ec16de80326b175"

func getMovie(c *gin.Context) {
	movieTitle := c.Query("title")
	if movieTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O parâmetro 'title' é obrigatório"})
		return
	}

	encodedTitle := url.QueryEscape(movieTitle)
	var movie Movie
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 2) // Canal para capturar erros

	wg.Add(2) // Definimos que vamos rodar duas goroutines

	// Requisição ao OMDb em paralelo
	go func() {
		defer wg.Done()
		apiURLOMDB := fmt.Sprintf("http://www.omdbapi.com/?t=%s&apiKey=%s", encodedTitle, apiKeyOMDB)
		resp, err := http.Get(apiURLOMDB)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errChan <- err
			return
		}

		var tempMovie Movie
		if err := json.Unmarshal(body, &tempMovie); err != nil {
			errChan <- err
			return
		}

		// Protegemos a escrita concorrente com Mutex
		mu.Lock()
		movie = tempMovie
		mu.Unlock()
	}()

	// Requisição ao TMDb em paralelo
	go func() {
		defer wg.Done()
		apiURLTMDB := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?api_key=%s&query=%s", apiKeyTMDB, encodedTitle)
		resp, err := http.Get(apiURLTMDB)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errChan <- err
			return
		}

		var tmdbResponse struct {
			Results []struct {
				ID          int    `json:"id"`
				Title       string `json:"title"`
				ReleaseDate string `json:"release_date"`
			} `json:"results"`
		}

		if err := json.Unmarshal(body, &tmdbResponse); err != nil {
			errChan <- err
			return
		}

		var movieID int
		for _, movieResult := range tmdbResponse.Results {
			if strings.HasPrefix(movieResult.ReleaseDate, "1996") {
				movieID = movieResult.ID
				break
			}
		}

		if movieID == 0 && len(tmdbResponse.Results) > 0 {
			movieID = tmdbResponse.Results[0].ID
		}

		if movieID != 0 {
			// Buscamos as avaliações do filme
			apiURLRatings := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/reviews?api_key=%s", movieID, apiKeyTMDB)
			respRatings, err := http.Get(apiURLRatings)
			if err != nil {
				errChan <- err
				return
			}
			defer respRatings.Body.Close()

			bodyRatings, err := ioutil.ReadAll(respRatings.Body)
			if err != nil {
				errChan <- err
				return
			}

			var reviewsResponse struct {
				Results []struct {
					Content string `json:"content"`
				} `json:"results"`
			}

			if err := json.Unmarshal(bodyRatings, &reviewsResponse); err != nil {
				errChan <- err
				return
			}

			// Coletamos até 3 comentários
			var comments []string
			for i := 0; i < 3 && i < len(reviewsResponse.Results); i++ {
				comments = append(comments, reviewsResponse.Results[i].Content)
			}

			// Protegemos a escrita concorrente com Mutex
			mu.Lock()
			movie.Comments = comments
			mu.Unlock()
		} else {
			mu.Lock()
			movie.Comments = []string{"Nenhuma avaliação encontrada"}
			mu.Unlock()
		}
	}()

	// Esperamos as goroutines terminarem
	wg.Wait()
	close(errChan) // Fechamos o canal de erros

	// Verificamos se houve erro
	for err := range errChan {
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, movie)
}

func main() {
	r := gin.Default()

	// Habilita CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Endpoint para buscar filmes
	r.GET("/movie", getMovie)

	// Iniciamos o servidor na porta 8080
	r.Run(":8080")
}
