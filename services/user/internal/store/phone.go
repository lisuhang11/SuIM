package store

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

func (s *Store) CreatePhoneUser(ctx context.Context, u *model.User) error {
	agreed := u.ProtocolAgreedAt
	if agreed.IsZero() {
		agreed = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user (
			user_id, email, password_hash, nickname, avatar_url,
			app_manger_level, global_recv_msg_opt, is_active,
			area_code, phone_number, register_type, ex,
			protocol_version, protocol_agreed_at, register_ip, register_receipt)
		VALUES (?, NULL, ?, ?, '', 0, 0, 1, ?, ?, ?, '', ?, ?, ?, ?)`,
		u.UserID, u.PasswordHash, u.Nickname,
		u.AreaCode, u.PhoneNumber, model.RegisterTypePhone,
		u.ProtocolVersion, agreed.UTC(), u.RegisterIP, u.RegisterReceipt,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			return ErrPhoneTaken
		}
		return err
	}
	got, err := s.GetUserByID(ctx, u.UserID)
	if err != nil {
		return err
	}
	*u = *got
	return nil
}
