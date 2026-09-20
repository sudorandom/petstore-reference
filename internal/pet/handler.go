package pet

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/db"
)

type Handler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

var _ petv1connect.PetServiceHandler = (*Handler)(nil)

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{
		pool:    pool,
		queries: db.New(pool),
	}
}

// New is a convenience alias for NewHandler.
func New(pool *pgxpool.Pool) *Handler {
	return NewHandler(pool)
}

func parseDate(dateStr string) (pgtype.Date, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return pgtype.Date{Valid: false}, nil
	}
	t, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return pgtype.Date{}, errors.New("invalid birth_date format, expected YYYY-MM-DD")
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func (h *Handler) CreatePet(ctx context.Context, req *connect.Request[petv1.CreatePetRequest]) (*connect.Response[petv1.CreatePetResponse], error) {
	msg := req.Msg

	name := strings.TrimSpace(msg.Name)
	species := strings.TrimSpace(msg.Species)
	if name == "" || species == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and species cannot be blank"))
	}

	tags := msg.Tags
	if tags == nil {
		tags = []string{}
	}

	photoUrls := msg.PhotoUrls
	if photoUrls == nil {
		photoUrls = []string{}
	}

	callerEmail, ok := auth.UserEmailFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authenticated identity has no email"))
	}

	birthDate, err := parseDate(msg.BirthDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	status := msg.Status
	if status == petv1.PetStatus_PET_STATUS_UNSPECIFIED {
		status = petv1.PetStatus_PET_STATUS_AVAILABLE
	}

	created, err := h.queries.CreatePet(ctx, db.CreatePetParams{
		Name:               name,
		Species:            species,
		BirthDate:          birthDate,
		BirthDateEstimated: msg.BirthDateEstimated,
		Status:             status.String(),
		PhotoUrls:          photoUrls,
		Tags:               tags,
		CreatedBy:          callerEmail,
		ModifiedBy:         callerEmail,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&petv1.CreatePetResponse{
		Pet: toProtoPet(created),
	}), nil
}

func (h *Handler) GetPet(ctx context.Context, req *connect.Request[petv1.GetPetRequest]) (*connect.Response[petv1.GetPetResponse], error) {
	var uid pgtype.UUID
	if err := uid.Scan(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	item, err := h.queries.GetPet(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("pet not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&petv1.GetPetResponse{
		Pet: toProtoPet(item),
	}), nil
}

func (h *Handler) ListPets(ctx context.Context, req *connect.Request[petv1.ListPetsRequest]) (*connect.Response[petv1.ListPetsResponse], error) {
	msg := req.Msg

	limit := int32(20)
	if msg.PageSize > 0 {
		limit = msg.PageSize
	}
	offset := int64(0)
	if msg.Page > 0 {
		offset = int64(msg.Page) * int64(limit)
		if offset > math.MaxInt32 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page offset is too large"))
		}
	}

	var statusParam pgtype.Text
	if msg.Status != petv1.PetStatus_PET_STATUS_UNSPECIFIED {
		statusParam = pgtype.Text{String: msg.Status.String(), Valid: true}
	}

	var speciesParam pgtype.Text
	if msg.Species != "" {
		speciesParam = pgtype.Text{String: msg.Species, Valid: true}
	}

	pets, err := h.queries.ListPets(ctx, db.ListPetsParams{
		Limit:   limit,
		Offset:  int32(offset),
		Status:  statusParam,
		Species: speciesParam,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	totalCount, err := h.queries.CountPets(ctx, db.CountPetsParams{
		Status:  statusParam,
		Species: speciesParam,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoPets := make([]*petv1.Pet, len(pets))
	for i, p := range pets {
		protoPets[i] = toProtoPet(p)
	}

	var totalCount32 int32
	if totalCount > math.MaxInt32 {
		totalCount32 = math.MaxInt32
	} else if totalCount < math.MinInt32 {
		totalCount32 = math.MinInt32
	} else {
		totalCount32 = int32(totalCount)
	}

	return connect.NewResponse(&petv1.ListPetsResponse{
		Pets:       protoPets,
		TotalCount: totalCount32,
	}), nil
}

func (h *Handler) UpdatePet(ctx context.Context, req *connect.Request[petv1.UpdatePetRequest]) (*connect.Response[petv1.UpdatePetResponse], error) {
	msg := req.Msg

	var uid pgtype.UUID
	if err := uid.Scan(msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	name := strings.TrimSpace(msg.Name)
	species := strings.TrimSpace(msg.Species)
	if name == "" || species == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and species cannot be blank"))
	}

	tags := msg.Tags
	if tags == nil {
		tags = []string{}
	}

	photoUrls := msg.PhotoUrls
	if photoUrls == nil {
		photoUrls = []string{}
	}

	callerEmail, ok := auth.UserEmailFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("authenticated identity has no email"))
	}

	birthDate, err := parseDate(msg.BirthDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	status := msg.Status
	if status == petv1.PetStatus_PET_STATUS_UNSPECIFIED {
		status = petv1.PetStatus_PET_STATUS_AVAILABLE
	}

	updated, err := h.queries.UpdatePet(ctx, db.UpdatePetParams{
		ID:                 uid,
		Name:               name,
		Species:            species,
		BirthDate:          birthDate,
		BirthDateEstimated: msg.BirthDateEstimated,
		Status:             status.String(),
		PhotoUrls:          photoUrls,
		Tags:               tags,
		ModifiedBy:         callerEmail,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("pet not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&petv1.UpdatePetResponse{
		Pet: toProtoPet(updated),
	}), nil
}

func (h *Handler) DeletePet(ctx context.Context, req *connect.Request[petv1.DeletePetRequest]) (*connect.Response[petv1.DeletePetResponse], error) {
	var uid pgtype.UUID
	if err := uid.Scan(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	rowsAffected, err := h.queries.DeletePet(ctx, uid)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if rowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("pet not found"))
	}

	return connect.NewResponse(&petv1.DeletePetResponse{
		Success: true,
	}), nil
}

func toProtoPet(p db.Pet) *petv1.Pet {
	statusStr := p.Status
	if !strings.HasPrefix(statusStr, "PET_STATUS_") {
		statusStr = "PET_STATUS_" + statusStr
	}
	statusVal := petv1.PetStatus_value[statusStr]

	petID := uuid.UUID(p.ID.Bytes).String()

	var birthDateStr string
	if p.BirthDate.Valid {
		birthDateStr = p.BirthDate.Time.Format("2006-01-02")
	}

	protoPet := &petv1.Pet{
		Id:                 petID,
		Name:               p.Name,
		Species:            p.Species,
		BirthDate:          birthDateStr,
		BirthDateEstimated: p.BirthDateEstimated,
		Status:             petv1.PetStatus(statusVal),
		PhotoUrls:          p.PhotoUrls,
		Tags:               p.Tags,
		CreatedBy:          p.CreatedBy,
		ModifiedBy:         p.ModifiedBy,
	}

	if p.CreatedAt.Valid {
		protoPet.CreatedAt = timestamppb.New(p.CreatedAt.Time)
	}
	if p.ModifiedAt.Valid {
		protoPet.ModifiedAt = timestamppb.New(p.ModifiedAt.Time)
	}

	return protoPet
}
