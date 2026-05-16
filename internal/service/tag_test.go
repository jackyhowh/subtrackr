package service

import (
	"reflect"
	"subtrackr/internal/models"
	"subtrackr/internal/repository"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestParseTagsInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string yields nothing", "", []string{}},
		{"single tag", "work", []string{"work"}},
		{"comma separated", "work,family,autopay", []string{"work", "family", "autopay"}},
		{"trims whitespace around tags", "  work , family ,  autopay  ", []string{"work", "family", "autopay"}},
		{"drops empty entries between commas", "work,,family,,,autopay", []string{"work", "family", "autopay"}},
		{"keeps case as-typed (FindOrCreate handles dedupe)", "Work,WORK,work", []string{"Work", "WORK", "work"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseTagsInput(tc.input)
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseTagsInput(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func setupTagTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, db.AutoMigrate(&models.Tag{}, &models.Category{}, &models.Subscription{}))
	// many2many join table — mirror what migrations.go does at boot
	assert.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS subscription_tags (
			subscription_id INTEGER NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (subscription_id, tag_id)
		)
	`).Error)
	return db
}

func TestFindOrCreateByNames_DedupesCaseInsensitive(t *testing.T) {
	db := setupTagTestDB(t)
	repo := repository.NewTagRepository(db)

	// Submit duplicates with mixed case and whitespace — should collapse to 3 unique tags.
	tags, err := repo.FindOrCreateByNames([]string{"Work", "WORK", "work", " family ", "Family", "autopay", ""})
	assert.NoError(t, err)
	assert.Len(t, tags, 3)

	// Second call with overlapping names must not create duplicates in DB.
	_, err = repo.FindOrCreateByNames([]string{"work", "family", "new-tag"})
	assert.NoError(t, err)

	var count int64
	db.Model(&models.Tag{}).Count(&count)
	assert.EqualValues(t, 4, count, "should have only 4 unique tags after re-submission")
}

func TestFindOrCreateByNames_EmptyInputReturnsEmpty(t *testing.T) {
	db := setupTagTestDB(t)
	repo := repository.NewTagRepository(db)

	tags, err := repo.FindOrCreateByNames([]string{})
	assert.NoError(t, err)
	assert.Empty(t, tags)

	tags, err = repo.FindOrCreateByNames([]string{"", "  ", "\t"})
	assert.NoError(t, err)
	assert.Empty(t, tags)
}

func TestSetSubscriptionTags_ReplaceSemantics(t *testing.T) {
	db := setupTagTestDB(t)
	tagRepo := repository.NewTagRepository(db)
	svc := NewTagService(tagRepo)

	// Need a category for FK
	cat := &models.Category{Name: "Streaming"}
	assert.NoError(t, db.Create(cat).Error)

	sub := &models.Subscription{Name: "Netflix", Cost: 15.99, Schedule: "Monthly", Status: "Active", CategoryID: cat.ID}
	assert.NoError(t, db.Create(sub).Error)

	// Set initial tags
	_, err := svc.SetSubscriptionTags(sub.ID, []string{"work", "autopay", "important"})
	assert.NoError(t, err)

	loaded := loadSubWithTags(t, db, sub.ID)
	assert.ElementsMatch(t, []string{"work", "autopay", "important"}, tagNames(loaded.Tags))

	// Replace with a smaller set — old tags should be detached, "important" should be orphaned and cleaned
	_, err = svc.SetSubscriptionTags(sub.ID, []string{"work", "renamed"})
	assert.NoError(t, err)

	loaded = loadSubWithTags(t, db, sub.ID)
	assert.ElementsMatch(t, []string{"work", "renamed"}, tagNames(loaded.Tags))

	// Verify orphan cleanup removed "autopay" and "important" from the tags table
	var total int64
	db.Model(&models.Tag{}).Count(&total)
	assert.EqualValues(t, 2, total, "orphaned tags (autopay, important) should be cleaned up")
}

func TestSetSubscriptionTags_EmptyClearsAll(t *testing.T) {
	db := setupTagTestDB(t)
	tagRepo := repository.NewTagRepository(db)
	svc := NewTagService(tagRepo)

	cat := &models.Category{Name: "Streaming"}
	assert.NoError(t, db.Create(cat).Error)
	sub := &models.Subscription{Name: "Netflix", Cost: 15.99, Schedule: "Monthly", Status: "Active", CategoryID: cat.ID}
	assert.NoError(t, db.Create(sub).Error)

	_, err := svc.SetSubscriptionTags(sub.ID, []string{"work", "family"})
	assert.NoError(t, err)

	_, err = svc.SetSubscriptionTags(sub.ID, []string{})
	assert.NoError(t, err)

	loaded := loadSubWithTags(t, db, sub.ID)
	assert.Empty(t, loaded.Tags)

	// Orphan cleanup should have removed all
	var total int64
	db.Model(&models.Tag{}).Count(&total)
	assert.EqualValues(t, 0, total)
}

func loadSubWithTags(t *testing.T, db *gorm.DB, id uint) *models.Subscription {
	t.Helper()
	var sub models.Subscription
	assert.NoError(t, db.Preload("Tags").First(&sub, id).Error)
	return &sub
}

func tagNames(tags []models.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.Name
	}
	return out
}
