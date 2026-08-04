package catalog

import "github.com/otameshi/backend/internal/config"

type Product = config.Product

func GetProduct(id string) *Product {
	for _, p := range config.Get().Products {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

func GetProducts(ids []string) []Product {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	products := config.Get().Products
	result := make([]Product, 0, len(ids))
	for _, p := range products {
		if idSet[p.ID] {
			result = append(result, p)
		}
	}
	return result
}

func GetAllProducts() []Product {
	return config.Get().Products
}
