package tours

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string into a URL-friendly lowercase slug
func Slugify(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	slug := nonAlphaNumRegex.ReplaceAllString(text, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "tour"
	}
	return slug
}

// generateUniqueSlug ensures that the generated slug does not collide with existing tours
func generateUniqueSlug(db *gorm.DB, name string, excludeTourID ...string) string {
	baseSlug := Slugify(name)
	slug := baseSlug
	counter := 1

	for {
		var count int64
		query := db.Table("tours").Where("slug = ?", slug)
		if len(excludeTourID) > 0 && excludeTourID[0] != "" {
			query = query.Where("id != ?", excludeTourID[0])
		}
		query.Count(&count)
		if count == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		counter++
	}
}
