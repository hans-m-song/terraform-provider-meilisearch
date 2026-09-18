resource "meilisearch_index" "example" {
  uid = "index-settings-example"
}

resource "meilisearch_index_settings" "example" {
  index_uid = meilisearch_index.example.uid

  searchable_attributes = ["title", "description"]
  displayed_attributes  = ["id", "title", "description"]
  filterable_attributes = ["genre"]
  sortable_attributes   = ["release_date"]
  ranking_rules         = ["words", "typo", "proximity", "attributeRank", "wordPosition", "sort", "exactness"]
  stop_words            = ["the", "and"]
  synonyms = {
    "winnie" = ["pooh"]
  }
  distinct_attribute = "movie_id"
}
