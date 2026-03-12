package model

type SearchResponse struct {
	Items    []SearchItem  `json:"items"`
	Facets   []FacetResult `json:"facets,omitempty"`
	Page     PageInfo      `json:"page"`
	Meta     SearchMeta    `json:"meta"`
}

type SearchItem struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Score              float64  `json:"score"`
	Description        string   `json:"description,omitempty"`
	Headline           string   `json:"headline,omitempty"`
	Slug               string   `json:"slug,omitempty"`
	ImageURL           string   `json:"imageUrl,omitempty"`
	Locale             string   `json:"locale,omitempty"`
	Market             string   `json:"market,omitempty"`
	Categories         []string `json:"categories,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	Allergens          []string `json:"allergens,omitempty"`
	Ingredients        []string `json:"ingredients,omitempty"`
	RecipeCuisine      []string `json:"recipeCuisine,omitempty"`
	Utensils           []string `json:"utensils,omitempty"`
	ShoppingSegments   []string `json:"shoppingSegments,omitempty"`
	Active             bool     `json:"active"`
	SoldOut            bool     `json:"soldOut"`
	IsAddon            bool     `json:"isAddon"`
	IsHidden           bool     `json:"isHidden"`
	HideOnSoldOut      bool     `json:"hideOnSoldOut"`
	Week               string   `json:"week,omitempty"`
	TotalTime          string   `json:"totalTime,omitempty"`
	PreparationTime    string   `json:"preparationTime,omitempty"`
	DifficultyLevel    string   `json:"difficultyLevel,omitempty"`
	TotalCalories      string   `json:"totalCalories,omitempty"`
	Fat                string   `json:"fat,omitempty"`
	SaturatedFat       string   `json:"saturatedFat,omitempty"`
	Carbohydrate       string   `json:"carbohydrate,omitempty"`
	DietaryFiber       string   `json:"dietaryFiber,omitempty"`
	Sugar              string   `json:"sugar,omitempty"`
	Sodium             string   `json:"sodium,omitempty"`
	PriceType          string   `json:"priceType,omitempty"`
	Price              string   `json:"price,omitempty"`
	ParentID           string   `json:"parentId,omitempty"`
	Index              int      `json:"index,omitempty"`
	UpdatedAt          string   `json:"updatedAt,omitempty"`
	MenuID             string   `json:"menuId,omitempty"`
	RecipeID           string   `json:"recipeId,omitempty"`
	ShoppableProductID string   `json:"shoppableProductId,omitempty"`
}

type FacetResult struct {
	Field   string       `json:"field"`
	Buckets []FacetBucket `json:"buckets"`
}

type FacetBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type PageInfo struct {
	Size        int    `json:"size"`
	HasNextPage bool   `json:"hasNextPage"`
	Cursor      string `json:"cursor,omitempty"`
}

type SearchMeta struct {
	TotalHits int      `json:"totalHits"`
	Stage     string   `json:"stage"`
	Warnings  []string `json:"warnings,omitempty"`
}
