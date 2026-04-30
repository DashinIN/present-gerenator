package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/you/fungreet/internal/services"
)

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("SUNO_API_KEY")
	if apiKey == "" {
		fmt.Println("SUNO_API_KEY not set")
		os.Exit(1)
	}

	gen := services.NewSunoAPIGenerator(apiKey)

	lyrics := "рота субагентов солдат поздравляем вас!!!"
	style := "dance, hardstyle"

	fmt.Printf("Submitting: lyrics=%q style=%q\n", lyrics, style)

	ctx := context.Background()
	results, err := gen.Generate(ctx, lyrics, style, 1)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	for i, data := range results {
		path := fmt.Sprintf("test_song_%d.mp3", i)
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Printf("write error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Saved: %s (%d bytes)\n", path, len(data))
	}
}
