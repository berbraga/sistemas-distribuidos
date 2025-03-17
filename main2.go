package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
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

	// Estruturas para armazenar os dados das APIs
	var movie Movie
	var comments []string
	var mu sync.Mutex
	var eg errgroup.Group

	// Busca dados do OMDb
	eg.Go(func() error {
		apiURLOMDB := fmt.Sprintf("http://www.omdbapi.com/?t=%s&apiKey=%s", encodedTitle, apiKeyOMDB)
		resp, err := http.Get(apiURLOMDB)
		if err != nil {
			return fmt.Errorf("erro ao buscar dados do OMDb: %v", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("erro ao ler resposta do OMDb: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()

		// Convertemos a resposta JSON do OMDb para a struct Movie
		err = json.Unmarshal(body, &movie)
		if err != nil {
			return fmt.Errorf("erro ao converter resposta do OMDb: %v", err)
		}

		return nil
	})

	// Busca dados do TMDb
	eg.Go(func() error {
		apiURLTMDB := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?api_key=%s&query=%s", apiKeyTMDB, encodedTitle)
		resp, err := http.Get(apiURLTMDB)
		if err != nil {
			return fmt.Errorf("erro ao buscar dados do TMDb: %v", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("erro ao ler resposta do TMDb: %v", err)
		}

		var tmdbResponse struct {
			Results []struct {
				ID          int    `json:"id"`
				Title       string `json:"title"`
				ReleaseDate string `json:"release_date"`
			} `json:"results"`
		}

		if err := json.Unmarshal(body, &tmdbResponse); err != nil {
			return fmt.Errorf("erro ao processar resposta do TMDb: %v", err)
		}

		// Filtra filmes de 1996
		var filteredMovies []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			ReleaseDate string `json:"release_date"`
		}

		for _, movieResult := range tmdbResponse.Results {
			if strings.HasPrefix(movieResult.ReleaseDate, "1996") {
				filteredMovies = append(filteredMovies, movieResult)
			}
		}

		// Se não encontrar filme de 1996, usa o primeiro da lista
		if len(filteredMovies) == 0 && len(tmdbResponse.Results) > 0 {
			filteredMovies = append(filteredMovies, tmdbResponse.Results[0])
		}

		// Busca avaliações se encontrou um filme
		if len(filteredMovies) > 0 {
			movieID := filteredMovies[0].ID
			apiURLRatingsTMDB := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/reviews?api_key=%s", movieID, apiKeyTMDB)
			respRatings, err := http.Get(apiURLRatingsTMDB)
			if err != nil {
				return fmt.Errorf("erro ao buscar avaliações do TMDb: %v", err)
			}
			defer respRatings.Body.Close()

			bodyRatings, err := ioutil.ReadAll(respRatings.Body)
			if err != nil {
				return fmt.Errorf("erro ao ler avaliações do TMDb: %v", err)
			}

			var reviewsResponse struct {
				Results []struct {
					Content string `json:"content"`
				} `json:"results"`
			}

			if err := json.Unmarshal(bodyRatings, &reviewsResponse); err != nil {
				return fmt.Errorf("erro ao processar avaliações do TMDb: %v", err)
			}

			// Limita os comentários para 3
			mu.Lock()
			defer mu.Unlock()
			for i := 0; i < 3 && i < len(reviewsResponse.Results); i++ {
				comments = append(comments, reviewsResponse.Results[i].Content)
			}
		}

		return nil
	})

	// Aguarda todas as goroutines finalizarem
	if err := eg.Wait(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Se não houver comentários, adiciona uma mensagem padrão
	if len(comments) == 0 {
		comments = []string{"Nenhuma avaliação encontrada"}
	}

	// Atualiza a struct do filme com os comentários
	movie.Comments = comments

	// Retorna os dados do filme
	c.JSON(http.StatusOK, movie)
}

func main() {
	r := gin.Default()

	// Habilita CORS para permitir requisições do front-end
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Permite qualquer origem (ajuste conforme necessário)
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Endpoint para buscar filmes pelo título
	r.GET("/movie", getMovie)

	// Inicia o servidor na porta 8080
	r.Run(":8080")
}
