package model

type BulkImportJob struct {
	ID          string  `json:"id" db:"id"`
	ActorID     string  `json:"actorId" db:"actor_id"`
	Status      string  `json:"status" db:"status"`
	Total       int     `json:"total" db:"total"`
	Processed   int     `json:"processed" db:"processed"`
	Failed      int     `json:"failed" db:"failed"`
	Errors      string  `json:"errors,omitempty" db:"errors"`
	CreatedAt   string  `json:"createdAt" db:"created_at"`
	CompletedAt *string `json:"completedAt,omitempty" db:"completed_at"`
}
