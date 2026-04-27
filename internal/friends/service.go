package friends

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"recommendation-system/internal/models"
	"recommendation-system/internal/repository"
)

var (
	ErrCannotFriendSelf        = errors.New("cannot send friend request to yourself")
	ErrTargetUserNotFound      = errors.New("target user not found")
	ErrInvalidAction           = errors.New("invalid action")
	ErrFriendRequestNotFound   = errors.New("friend request not found")
	ErrFriendRequestNotPending = errors.New("friend request is not pending")
)

func SendFriendRequest(ctx context.Context, fromUserID, toUserID uuid.UUID) error {
	if fromUserID == toUserID {
		return ErrCannotFriendSelf
	}

	if _, err := repository.GetUserByID(ctx, toUserID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrTargetUserNotFound
		}
		return err
	}

	outgoingStatus, outgoingFound, err := repository.GetFriendshipStatus(ctx, fromUserID, toUserID)
	if err != nil {
		return err
	}
	incomingStatus, incomingFound, err := repository.GetFriendshipStatus(ctx, toUserID, fromUserID)
	if err != nil {
		return err
	}

	if (outgoingFound && outgoingStatus == "accepted") || (incomingFound && incomingStatus == "accepted") {
		return repository.ErrFriendRequestAlreadyExists
	}
	if (outgoingFound && outgoingStatus == "pending") || (incomingFound && incomingStatus == "pending") {
		return repository.ErrFriendRequestAlreadyExists
	}

	if outgoingFound {
		if outgoingStatus == "rejected" {
			if err := repository.UpdateFriendRequestStatus(ctx, fromUserID, toUserID, "pending"); err != nil {
				return err
			}
			return nil
		}
	}

	return repository.CreateFriendRequest(ctx, fromUserID, toUserID)
}

func RespondToFriendRequest(ctx context.Context, receiverID uuid.UUID, fromUserID uuid.UUID, action string) (newStatus string, err error) {
	if action != "accept" && action != "reject" {
		return "", ErrInvalidAction
	}
	if fromUserID == receiverID {
		return "", ErrInvalidAction
	}

	status, found, err := repository.GetFriendshipStatus(ctx, fromUserID, receiverID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrFriendRequestNotFound
	}
	if status != "pending" {
		return "", ErrFriendRequestNotPending
	}

	if action == "accept" {
		if err := repository.UpdateFriendRequestStatus(ctx, fromUserID, receiverID, "accepted"); err != nil {
			return "", err
		}
		if err := repository.EnsureAcceptedMirror(ctx, fromUserID, receiverID); err != nil {
			return "", err
		}
		return "accepted", nil
	}

	if err := repository.UpdateFriendRequestStatus(ctx, fromUserID, receiverID, "rejected"); err != nil {
		return "", err
	}
	return "rejected", nil
}

func GetFriends(ctx context.Context, userID uuid.UUID) ([]models.User, error) {
	friendIDs, err := repository.ListAcceptedFriendIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	friends, err := repository.GetUsersByIDs(ctx, friendIDs)
	if err != nil {
		return nil, err
	}

	return friends, nil
}

func ListIncomingFriendRequests(ctx context.Context, userID uuid.UUID) ([]models.IncomingFriendRequest, error) {
	return repository.ListIncomingFriendRequests(ctx, userID)
}

func ListOutgoingFriendRequests(ctx context.Context, userID uuid.UUID) ([]models.OutgoingFriendRequest, error) {
	return repository.ListOutgoingFriendRequests(ctx, userID)
}
