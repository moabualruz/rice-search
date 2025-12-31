package evaluation

import (
	"encoding/json"
	"net/http"
)

// Handler provides HTTP handlers for evaluation.
type Handler struct {
	evaluator *Evaluator
}

// NewHandler creates a new evaluation handler.
func NewHandler(e *Evaluator) *Handler {
	return &Handler{evaluator: e}
}

// RegisterRoutes registers evaluation routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/evaluation/evaluate", h.handleEvaluate)
	mux.HandleFunc("POST /v1/evaluation/judgments", h.handleLoadJudgments)
}

type EvaluateRequest struct {
	Queries []struct {
		ID    string `json:"id"`
		Query string `json:"query"`
	} `json:"queries"`
	Store string `json:"store"`
	Ks    []int  `json:"ks"`
}

type EvaluateResponse struct {
	Results []*EvaluationResult `json:"results"`
	Summary *EvaluationSummary  `json:"summary"`
}

func (h *Handler) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Store == "" {
		http.Error(w, "store is required", http.StatusBadRequest)
		return
	}
	if len(req.Ks) == 0 {
		req.Ks = []int{1, 3, 5, 10} // default Ks
	}

	results := make([]*EvaluationResult, 0, len(req.Queries))
	ctx := r.Context()

	for _, q := range req.Queries {
		res, err := h.evaluator.EvaluateQuery(ctx, q.ID, q.Query, req.Store, req.Ks)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, res)
	}

	summary := h.evaluator.Summarize(results)
	resp := EvaluateResponse{
		Results: results,
		Summary: summary,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleLoadJudgments(w http.ResponseWriter, r *http.Request) {
	var judgments []RelevanceJudgment
	if err := json.NewDecoder(r.Body).Decode(&judgments); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.evaluator.LoadJudgments(judgments)
	w.WriteHeader(http.StatusNoContent)
}
