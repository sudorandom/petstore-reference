package pet

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	var birthDate pgtype.Date
	t, err := time.Parse("2006-01-02", msg.BirthDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid birth_date format, expected YYYY-MM-DD"))
	}
	birthDate = pgtype.Date{Time: t, Valid: true}

	created, err := s.queries.CreatePet(ctx, db.CreatePetParams{
		Name:               msg.Name,
		Species:            msg.Species,
		BirthDate:          birthDate,
		BirthDateEstimated: msg.BirthDateEstimated,
		Status:             msg.Status.String(),
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

	var birthDate pgtype.Date
	t, err := time.Parse("2006-01-02", msg.BirthDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid birth_date format, expected YYYY-MM-DD"))
	}
	birthDate = pgtype.Date{Time: t, Valid: true}

	updated, err := s.queries.UpdatePet(ctx, db.UpdatePetParams{
		ID:                 uid,
		Name:               msg.Name,
		Species:            msg.Species,
		BirthDate:          birthDate,
		BirthDateEstimated: msg.BirthDateEstimated,
		Status:             msg.Status.String(),
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

func (s *Service) UploadPetPhoto(ctx context.Context, req *connect.Request[petv1.UploadPetPhotoRequest]) (*connect.Response[petv1.UploadPetPhotoResponse], error) {
	msg := req.Msg

	var petUID pgtype.UUID
	if err := petUID.Scan(msg.PetId); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid pet UUID"))
	}

	// Verify pet exists
	_, err := s.queries.GetPet(ctx, petUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("pet not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	callerEmail := auth.UserEmailFromContext(ctx)

	if len(msg.Data) > 5*1024*1024 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("photo exceeds maximum allowed size"))
	}

	createdPhoto, err := s.queries.CreatePetPhoto(ctx, db.CreatePetPhotoParams{
		PetID:     petUID,
		Data:      msg.Data,
		MimeType:  msg.MimeType,
		SizeBytes: int32(len(msg.Data)), //nolint:gosec // G115: verified <= 5MB
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to store pet photo: %w", err))
	}

	photoID := uuid.UUID(createdPhoto.ID.Bytes).String()
	photoURL := fmt.Sprintf("/photos/%s", photoID)

	updatedPet, err := s.queries.AddPetPhotoURL(ctx, db.AddPetPhotoURLParams{
		ID:         petUID,
		PhotoUrl:   photoURL,
		ModifiedBy: callerEmail,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update pet photo list: %w", err))
	}

	return connect.NewResponse(&petv1.UploadPetPhotoResponse{
		PhotoId:  photoID,
		PhotoUrl: photoURL,
		Pet:      toProtoPet(updatedPet),
	}), nil
}

func (s *Service) GetPetPhoto(ctx context.Context, req *connect.Request[petv1.GetPetPhotoRequest]) (*connect.Response[petv1.GetPetPhotoResponse], error) {
	var photoUID pgtype.UUID
	if err := photoUID.Scan(req.Msg.PhotoId); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid photo UUID"))
	}

	photo, err := s.queries.GetPetPhoto(ctx, photoUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("photo not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&petv1.GetPetPhotoResponse{
		PhotoId:  uuid.UUID(photo.ID.Bytes).String(),
		PetId:    uuid.UUID(photo.PetID.Bytes).String(),
		Data:     photo.Data,
		MimeType: photo.MimeType,
	}), nil
}

func toProtoPet(p db.Pet) *petv1.Pet {
	statusVal := petv1.PetStatus_value[p.Status]

	petID := uuid.UUID(p.ID.Bytes).String()

	var birthDateStr string
	if p.BirthDate.Valid {
		birthDateStr = p.BirthDate.Time.Format("2006-01-02")
	}

	protoPet := &petv1.Pet{
		Id:                  petID,
		Name:                p.Name,
		Species:             p.Species,
		BirthDate:           birthDateStr,
		BirthDateEstimated:  p.BirthDateEstimated,
		Status:              petv1.PetStatus(statusVal),
		PhotoUrls:           p.PhotoUrls,
		Tags:                p.Tags,
		CreatedBy:           p.CreatedBy,
		ModifiedBy:          p.ModifiedBy,
	}

	if p.CreatedAt.Valid {
		protoPet.CreatedAt = timestamppb.New(p.CreatedAt.Time)
	}
	if p.ModifiedAt.Valid {
		protoPet.ModifiedAt = timestamppb.New(p.ModifiedAt.Time)
	}

	return protoPet
}

// NewPhotoHandler returns an http.Handler that serves pet photos by ID at /photos/{id}.
func NewPhotoHandler(queries *db.Queries) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/photos/")
		if idStr == "" {
			http.NotFound(w, r)
			return
		}
		var photoUID pgtype.UUID
		if err := photoUID.Scan(idStr); err != nil {
			http.Error(w, "Invalid photo ID", http.StatusBadRequest)
			return
		}
		if queries == nil {
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		photo, err := queries.GetPetPhoto(r.Context(), photoUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Failed to load photo", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", photo.MimeType)
		w.Header().Set("Content-Length", strconv.Itoa(len(photo.Data)))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(photo.Data)
	})
}
