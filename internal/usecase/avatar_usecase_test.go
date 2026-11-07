package usecase_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/primestrider/golang-clean-architecture/internal/apperror"
	"github.com/primestrider/golang-clean-architecture/internal/entity"
	"github.com/primestrider/golang-clean-architecture/internal/usecase"
	"github.com/primestrider/golang-clean-architecture/internal/usecase/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

type avatarFixture struct {
	users   *mocks.UserRepository
	storage *mocks.ObjectStorage
	tx      *fakeTransactor
	uc      *usecase.AvatarUseCase
}

func newAvatarFixture(t *testing.T) *avatarFixture {
	f := &avatarFixture{users: mocks.NewUserRepository(t), storage: mocks.NewObjectStorage(t), tx: &fakeTransactor{}}
	f.uc = usecase.NewAvatarUseCase(discardLogger(), f.tx, f.users, f.storage, 15*time.Minute)
	return f
}

func TestUpload_RejectsTextRenamedPNG(t *testing.T) {
	f := newAvatarFixture(t)
	body := strings.NewReader("hello, this is not an image")

	_, err := f.uc.Upload(context.Background(), "u1", body, int64(body.Len()))

	assert.Equal(t, apperror.CodeUnsupportedMedia, codeOf(err))
}

func TestUpload_TooLarge(t *testing.T) {
	f := newAvatarFixture(t)

	_, err := f.uc.Upload(context.Background(), "u1", strings.NewReader("x"), 2<<20+1)

	assert.Equal(t, apperror.CodePayloadTooLarge, codeOf(err))
}

func TestUpload_ReplacesAndDeletesOldAfterCommit(t *testing.T) {
	f := newAvatarFixture(t)
	old := "avatars/u1/old.png"
	data := pngBytes(t)
	var newKey string
	f.storage.EXPECT().Put(mock.Anything, mock.MatchedBy(func(key string) bool {
		newKey = key
		return strings.HasPrefix(key, "avatars/u1/") && strings.HasSuffix(key, ".png")
	}), mock.Anything, int64(len(data)), "image/png").RunAndReturn(func(_ context.Context, _ string, r io.Reader, _ int64, _ string) error {
		stored, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, data, stored, "the full file, including the sniffed header, must be stored")
		return nil
	})
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(&entity.User{ID: "u1", AvatarKey: &old}, nil)
	f.users.EXPECT().Update(mock.Anything, mock.MatchedBy(func(u *entity.User) bool { return u.AvatarKey != nil && *u.AvatarKey == newKey })).Return(nil)
	f.storage.EXPECT().Delete(mock.Anything, old).RunAndReturn(func(context.Context, string) error {
		assert.False(t, f.tx.inTx, "old avatar is deleted only after the transaction commits")
		return nil
	}).Once()
	f.storage.EXPECT().PresignGet(mock.Anything, mock.Anything, 15*time.Minute).Return("https://storage/signed", nil)

	res, err := f.uc.Upload(context.Background(), "u1", bytes.NewReader(data), int64(len(data)))

	require.NoError(t, err)
	assert.Equal(t, "https://storage/signed", res.AvatarURL)
}

func TestRemove_ClearsKeyAndDeletesObject(t *testing.T) {
	f := newAvatarFixture(t)
	old := "avatars/u1/old.png"
	f.users.EXPECT().FindByID(mock.Anything, "u1").Return(&entity.User{ID: "u1", AvatarKey: &old}, nil)
	f.users.EXPECT().Update(mock.Anything, mock.MatchedBy(func(u *entity.User) bool { return u.AvatarKey == nil })).Return(nil)
	f.storage.EXPECT().Delete(mock.Anything, old).Return(nil)

	require.NoError(t, f.uc.Remove(context.Background(), "u1"))
}
