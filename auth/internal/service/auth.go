package service

import (
	"context"
	"fmt"
	"github.com/MaksKazantsev/SSO/auth/internal/db"
	"github.com/MaksKazantsev/SSO/auth/internal/log"
	"github.com/MaksKazantsev/SSO/auth/internal/models"
	"github.com/MaksKazantsev/SSO/auth/internal/utils"
	"github.com/google/uuid"
	"math/rand"
	"strconv"
)

type Auth interface {
	Register(ctx context.Context, req models.RegReq) (models.RegRes, error)
	Login(ctx context.Context, req models.LogReq) (string, string, error)
	EmailSendCode(ctx context.Context, email string) error
	PasswordRecovery(ctx context.Context, cr models.Credentials) error
	EmailVerifyCode(ctx context.Context, code, email, t string) (string, string, error)
}

type auth struct {
	repo db.Repository
	smtp utils.Smtp
}

func NewAuth(repo db.Repository) Auth {
	return &auth{
		repo: repo,
		smtp: utils.NewSmtp(),
	}
}

func (a *auth) Register(ctx context.Context, req models.RegReq) (models.RegRes, error) {
	// logging
	log.GetLogger(ctx).Debug("uc layer success ✔")

	// hashing password
	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		return models.RegRes{}, err
	}
	req.Password = hashed

	// generating id
	req.UUID = uuid.New().String()

	// generating tokens
	rToken, err := utils.NewToken(req.UUID, utils.REFRESH)
	if err != nil {
		return models.RegRes{}, err
	}
	aToken, err := utils.NewToken(req.UUID, utils.ACCESS)
	if err != nil {
		return models.RegRes{}, err
	}
	req.Refresh = rToken

	// calling repo method
	if err = a.repo.Register(ctx, req); err != nil {
		return models.RegRes{}, fmt.Errorf("repo error: %w", err)
	}

	code := strconv.Itoa(rand.Intn(9001) + 1000)

	// sending code
	go func() {
		if err = a.smtp.SendCode(code, req.Email); err != nil {
			fmt.Printf("smtp error: %v\n", err)
		}
	}()

	// calling repo method
	if err = a.repo.EmailAddCode(ctx, code, req.Email); err != nil {
		return models.RegRes{}, fmt.Errorf("repo error: %w", err)
	}

	return models.RegRes{
		UUID:         req.UUID,
		RefreshToken: rToken,
		AccessToken:  aToken,
	}, nil
}

func (a *auth) Login(ctx context.Context, req models.LogReq) (string, string, error) {
	// logging
	log.GetLogger(ctx).Debug("uc layer success ✔")

	// get password
	res, err := a.repo.GetHashAndID(ctx, req.Email)
	if err != nil {
		return "", " ", fmt.Errorf("repo error: %w", err)
	}

	// compare passwords
	if err = utils.ComparePass(res.Password, req.Password); err != nil {
		return "", " ", fmt.Errorf("error: %w", err)
	}

	// generating token
	rToken, err := utils.NewToken(res.UUID, utils.REFRESH)
	if err != nil {
		return " ", " ", err
	}
	aToken, err := utils.NewToken(res.UUID, utils.ACCESS)
	if err != nil {
		return " ", " ", err
	}
	req.Refresh = rToken

	// login
	err = a.repo.Login(ctx, req)
	if err != nil {
		return "", " ", fmt.Errorf("repo error: %w", err)
	}

	return rToken, aToken, nil
}

func (a *auth) PasswordRecovery(ctx context.Context, cr models.Credentials) error {
	// logging
	log.GetLogger(ctx).Debug("uc layer success ✔")

	// hashing password
	hashed, err := utils.HashPassword(cr.Password)
	if err != nil {
		return fmt.Errorf("repo error: %w", err)
	}
	cr.Password = hashed

	// calling repo method
	err = a.repo.PasswordRecovery(ctx, cr)
	if err != nil {
		return fmt.Errorf("repo error: %w", err)
	}
	return nil
}

func (a *auth) EmailSendCode(ctx context.Context, email string) error {
	// logging
	log.GetLogger(ctx).Debug("uc layer success ✔")

	// code
	code := strconv.Itoa(rand.Intn(9009) + 1000)

	// send code
	if err := a.smtp.SendCode(code, email); err != nil {
		return fmt.Errorf("smtp error: %w", err)
	}

	// calling repo method
	if err := a.repo.EmailAddCode(ctx, code, email); err != nil {
		return fmt.Errorf("repo errpr: %w", err)
	}
	return nil
}

func (a *auth) EmailVerifyCode(ctx context.Context, code, email, t string) (string, string, error) {
	// logging
	log.GetLogger(ctx).Debug("uc layer success ✔")

	// calling repo method
	id, err := a.repo.EmailVerifyCode(ctx, code, email, t)
	if err != nil {
		return "", "", fmt.Errorf("repo error: %w", err)
	}

	// generating tokes
	aToken, err := utils.NewToken(id, utils.ACCESS)
	if err != nil {
		return "", "", fmt.Errorf("failed to create token: %w", err)
	}
	rToken, err := utils.NewToken(id, utils.REFRESH)
	if err != nil {
		return "", "", fmt.Errorf("failed to create token: %w", err)
	}

	// update refresh token
	if err = a.repo.UpdateRToken(ctx, id, rToken); err != nil {
		return "", "", fmt.Errorf("repo error: %w", err)
	}
	return aToken, rToken, nil
}
