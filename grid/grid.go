package grid

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func StartGrid() {
	// Create a Gin router with default middleware (logger, recovery, etc.)
	router := gin.Default()

	router.Any("/*any", func(c *gin.Context) {
		// Read the full request body
		bodyBytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
			return
		}

		// Log the HTTP method and registration details
		fmt.Printf("Received a %s request at /grid/register:\n", c.Request.Method)
		fmt.Println(string(bodyBytes))

		// Respond back with a generic JSON response
		c.JSON(http.StatusOK, gin.H{
			"status":  "OK",
			"message": "Request received",
		})
	})

	// Start the Gin server on port 4444.
	log.Println("Starting Selenium Grid stub on port 4444...")
	if err := router.Run(":4444"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
