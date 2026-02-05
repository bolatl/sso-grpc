package auth

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
	"sso/internal/domain/models"
	"sso/internal/lib/jwt"
	"sso/internal/lib/logger/sl"
	"sso/internal/storage"
	"time"
)

var (
	// ErrInvalidCredentials is returned when the provided credentials are invalid.
	ErrInvalidCredentials = fmt.Errorf("invalid credentials")
	ErrUserAlreadyExists  = fmt.Errorf("user already exists")
	ErrAppNotFound        = fmt.Errorf("application not found")
)

type Auth struct {
	log         *slog.Logger
	usrSaver    UserSaver
	usrProvider UserProvider
	appProvider AppProvider
	tokenTTL    time.Duration
}

type UserSaver interface {
	SaveUser(ctx context.Context, email string, passHash []byte) (uid int64, err error)
}

type UserProvider interface {
	User(ctx context.Context, email string) (models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
}

type AppProvider interface {
	App(ctx context.Context, appID int) (models.App, error)
}

// New creates a new instance of Auth service.
func New(
	log *slog.Logger,
	userSaver UserSaver,
	userProvider UserProvider,
	appProvider AppProvider,
	tokenTTL time.Duration,
) *Auth {
	return &Auth{
		log:         log,
		usrSaver:    userSaver,
		usrProvider: userProvider,
		appProvider: appProvider,
		tokenTTL:    tokenTTL,
	}
}

// Login checks the provided credentials and returns a token if they are valid.
func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appID int,
) (string, error) {
	const op = "auth.Login"
	log := a.log.With(slog.String("op", op), slog.String("email", email), slog.Int("app_id", appID))
	log.Info("attempting user login")

	user, err := a.usrProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			a.log.Warn("user not found", sl.Err(err))
			return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}
		log.Error("failed to get user", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if err = bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		log.Warn("invalid password attempt")
		return "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Warn("application not found", sl.Err(err))
			return "", fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}
		log.Error("failed to get application", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in successfully")
	token, err := jwt.NewToken(user, app, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate token", sl.Err(err))
		return "", fmt.Errorf("%s: %w", op, err)
	}
	return token, nil
}

// RegisterNewUser creates a new user with the provided email and password, and returns the new user's ID.
func (a *Auth) RegisterNewUser(
	ctx context.Context,
	email string,
	pass string,
) (int64, error) {
	op := "auth.RegisterNewUser"
	log := a.log.With(slog.String("op", op), slog.String("email", email))
	log.Info("registering new user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to hash password", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	id, err := a.usrSaver.SaveUser(ctx, email, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			log.Warn("user already exists", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", op, ErrUserAlreadyExists)
		}
		log.Error("failed to save new user", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	log.Info("new user registered", slog.Int64("user_id", id))
	return id, nil
}

// IsAdmin checks if the user with the given ID has admin privileges.
func (a *Auth) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	op := "auth.IsAdmin"
	log := a.log.With(slog.String("op", op), slog.Int64("user_id", userID))
	log.Info("checking if user is admin")

	isAdmin, err := a.usrProvider.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Warn("user not found", sl.Err(err))
			return false, fmt.Errorf("%s: %w", op, ErrAppNotFound)
		}
		log.Error("failed to check admin status", sl.Err(err))
		return false, fmt.Errorf("%s: %w", op, err)
	}
	log.Info("admin status checked", slog.Bool("is_admin", isAdmin))
	return isAdmin, nil
}
