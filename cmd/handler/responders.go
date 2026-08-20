package handler

import (
	"encoding/json"

	database "github.com/Aergiaaa/noted/internal/database"
)

func parseFinancePocket(content []byte) string {
	var c struct {
		Finance struct {
			Pocket string `json:"pocket"`
		} `json:"finance"`
	}
	if err := json.Unmarshal(content, &c); err != nil {
		return ""
	}

	return c.Finance.Pocket
}

type pageItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func toPageItems(rows []database.GetPagePaginatedRow) []pageItem {
	items := make([]pageItem, len(rows))
	for i, r := range rows {
		items[i] = pageItem{ID: r.ID.String(), Title: r.Title}
	}
	return items
}
