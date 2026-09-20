package pet

import (
	"context"
	"errors"
	"math"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
	"github.com/example/pets/internal/auth"
	"github.com/example/pets/internal/db"
)

type Service struct {
	queries *db.Queries
}

var _ petv1connect.PetServiceHandler = (*Service)(nil)

func NewService(queries *db.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

func (s *Service) CreatePet(ctx context.Context, req *connect.Request[petv1.CreatePetRequest]) (*connect.Response[petv1.CreatePetResponse], error) {
	msg := req.Msg

	photoUrls := msg.PhotoUrls
	if photoUrls == nil {
		photoUrls = []string{}
	}
	tags := msg.Tags
	if tags == nil {
		tags = []string{}
	}

	callerEmail := auth.UserEmailFromContext(ctx)

	created, err := s.queries.CreatePet(ctx, db.CreatePetParams{
		Name:       msg.Name,
		Species:    msg.Species,
		Age:        msg.Age,
		Status:     msg.Status.String(),
		PhotoUrls:  photoUrls,
		Tags:       tags,
		CreatedBy:  callerEmail,
		ModifiedBy: callerEmail,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&petv1.CreatePetResponse{
		Pet: toProtoPet(created),
	}), nil
}

func (s *Service) GetPet(ctx context.Context, req *connect.Request[petv1.GetPetRequest]) (*connect.Response[petv1.GetPetResponse], error) {
	var uid pgtype.UUID
	if err := uid.Scan(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	item, err := s.queries.GetPet(ctx, uid)
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

func (s *Service) ListPets(ctx context.Context, req *connect.Request[petv1.ListPetsRequest]) (*connect.Response[petv1.ListPetsResponse], error) {
	msg := req.Msg

	limit := int32(20)
	if msg.PageSize > 0 {
		limit = msg.PageSize
	}
	offset := int32(0)
	if msg.Page > 0 {
		offset = msg.Page * limit
	}

	var statusParam pgtype.Text
	if msg.Status != petv1.PetStatus_PET_STATUS_UNSPECIFIED {
		statusParam = pgtype.Text{String: msg.Status.String(), Valid: true}
	}

	var speciesParam pgtype.Text
	if msg.Species != "" {
		speciesParam = pgtype.Text{String: msg.Species, Valid: true}
	}

	pets, err := s.queries.ListPets(ctx, db.ListPetsParams{
		Limit:   limit,
		Offset:  offset,
		Status:  statusParam,
		Species: speciesParam,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	totalCount, err := s.queries.CountPets(ctx, db.CountPetsParams{
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

func (s *Service) UpdatePet(ctx context.Context, req *connect.Request[petv1.UpdatePetRequest]) (*connect.Response[petv1.UpdatePetResponse], error) {
	msg := req.Msg

	var uid pgtype.UUID
	if err := uid.Scan(msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	photoUrls := msg.PhotoUrls
	if photoUrls == nil {
		photoUrls = []string{}
	}
	tags := msg.Tags
	if tags == nil {
		tags = []string{}
	}

	callerEmail := auth.UserEmailFromContext(ctx)

	updated, err := s.queries.UpdatePet(ctx, db.UpdatePetParams{
		ID:         uid,
		Name:       msg.Name,
		Species:    msg.Species,
		Age:        msg.Age,
		Status:     msg.Status.String(),
		PhotoUrls:  photoUrls,
		Tags:       tags,
		ModifiedBy: callerEmail,
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

func (s *Service) DeletePet(ctx context.Context, req *connect.Request[petv1.DeletePetRequest]) (*connect.Response[petv1.DeletePetResponse], error) {
	var uid pgtype.UUID
	if err := uid.Scan(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	if err := s.queries.DeletePet(ctx, uid); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&petv1.DeletePetResponse{
		Success: true,
	}), nil
}

func toProtoPet(p db.Pet) *petv1.Pet {
	statusVal := petv1.PetStatus_value[p.Status]

	petID := uuid.UUID(p.ID.Bytes).String()

	protoPet := &petv1.Pet{
		Id:         petID,
		Name:       p.Name,
		Species:    p.Species,
		Age:        p.Age,
		Status:     petv1.PetStatus(statusVal),
		PhotoUrls:  p.PhotoUrls,
		Tags:       p.Tags,
		CreatedBy:  p.CreatedBy,
		ModifiedBy: p.ModifiedBy,
	}

	if p.CreatedAt.Valid {
		protoPet.CreatedAt = timestamppb.New(p.CreatedAt.Time)
	}
	if p.ModifiedAt.Valid {
		protoPet.ModifiedAt = timestamppb.New(p.ModifiedAt.Time)
	}

	return protoPet
}
