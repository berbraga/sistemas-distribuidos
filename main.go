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

// Substitua pela sua chave de API do OMDb
const apiKeyOMDB = "db87d64c"
const apiKeyTMDB = "10642f172ea2a7371ec16de80326b175"

func getMovie(c *gin.Context) {
	// Pegamos o título do filme da URL
	movieTitle := c.Query("title")

	// Se o título estiver vazio, retorna erro
	if movieTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "O parâmetro 'title' é obrigatório"})
		return
	}

	// Codifica o título para URL (ex: "star wars" → "star+wars")
	encodedTitle := url.QueryEscape(movieTitle)

	// Construímos a URL da API do OMDb
	apiURLOMDB := fmt.Sprintf("http://www.omdbapi.com/?t=%s&apiKey=%s", encodedTitle, apiKeyOMDB)

	// Fazemos a requisição HTTP GET para OMDb
	respOMDB, err := http.Get(apiURLOMDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar dados do OMDb"})
		return
	}
	defer respOMDB.Body.Close()

	// Lemos o corpo da resposta do OMDb
	bodyOMDB, err := ioutil.ReadAll(respOMDB.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao ler resposta OMDb"})
		return
	}

	// Convertemos a resposta JSON do OMDb para a struct Movie
	var movie Movie
	err = json.Unmarshal(bodyOMDB, &movie)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao converter dados OMDb"})
		return
	}

	// Agora, buscamos as avaliações no TMDb
	apiURLTMDB := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?api_key=%s&query=%s", apiKeyTMDB, encodedTitle)
	respTMDB, err := http.Get(apiURLTMDB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar dados do TMDb"})
		return
	}
	defer respTMDB.Body.Close()

	// Lemos o corpo da resposta do TMDb
	bodyTMDB, err := ioutil.ReadAll(respTMDB.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao ler resposta TMDb"})
		return
	}

	// Log para depuração - Verifique a resposta do TMDb
	fmt.Println("Resposta TMDb:", string(bodyTMDB)) // Exibe a resposta para ver o conteúdo retornado

	// Estrutura para armazenar a resposta do TMDb
	var tmdbResponse struct {
		Results []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			ReleaseDate string `json:"release_date"`
		} `json:"results"`
	}

	err = json.Unmarshal(bodyTMDB, &tmdbResponse)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao processar resposta do TMDb"})
		return
	}

	// Filtra os filmes de 1996
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

	// Se não encontrar filme de 1996, utiliza o primeiro da lista
	if len(filteredMovies) == 0 && len(tmdbResponse.Results) > 0 {
		filteredMovies = append(filteredMovies, tmdbResponse.Results[0])
	}

	// Verifica se encontrou algum filme filtrado
	if len(filteredMovies) > 0 {
		movieID := filteredMovies[0].ID
		fmt.Println("ID do filme encontrado no TMDb:", movieID) // Exibe o ID do filme encontrado

		// Agora, buscamos as avaliações do filme com base no ID
		apiURLRatingsTMDB := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/reviews?api_key=%s", movieID, apiKeyTMDB)
		respRatingsTMDB, err := http.Get(apiURLRatingsTMDB)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar avaliações do TMDb"})
			return
		}
		defer respRatingsTMDB.Body.Close()

		// Lemos o corpo da resposta das avaliações
		bodyRatingsTMDB, err := ioutil.ReadAll(respRatingsTMDB.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao ler avaliações do TMDb"})
			return
		}

		// Log para depuração - Verifique a resposta das avaliações
		fmt.Println("Resposta de avaliações TMDb:", string(bodyRatingsTMDB)) // Exibe as avaliações

		// Estrutura para processar as avaliações
		var reviewsResponse struct {
			Results []struct {
				Content string `json:"content"`
			} `json:"results"`
		}

		err = json.Unmarshal(bodyRatingsTMDB, &reviewsResponse)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao processar avaliações do TMDb"})
			return
		}

		// Limitar a quantidade de comentários para 3
		var comments []string
		for i := 0; i < 3 && i < len(reviewsResponse.Results); i++ {
			comments = append(comments, reviewsResponse.Results[i].Content)
		}

		// Atualiza a lista de comentários
		movie.Comments = comments
	} else {
		movie.Comments = []string{"Nenhuma avaliação encontrada"}
	}

	// Retornamos os dados do filme com as avaliações
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

	// Iniciamos o servidor na porta 8080
	r.Run(":8080")
}
