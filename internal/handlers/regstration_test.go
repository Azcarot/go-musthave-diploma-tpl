package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mock_storage "github.com/Azcarot/GopherMarketProject/internal/mock"
	"github.com/Azcarot/GopherMarketProject/internal/storage"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRegistration(t *testing.T) {
	type testing struct {
		userData     storage.UserData
		expStatus    int
		wrongRequest bool
	}
	var testData []testing
	userDT := storage.UserData{}
	userDT.Date = time.Now().Format(time.RFC3339)
	testData = append(testData, testing{userDT, 200, false})
	testData = append(testData, testing{userDT, 400, true})
	var ReqData storage.RegisterRequest
	for _, test := range testData {
		ReqData.Login = userDT.Login
		ReqData.Password = userDT.Password
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mock := mock_storage.NewMockPgxStorage(ctrl)
		storage.ST = mock
		if !test.wrongRequest {
			mock.EXPECT().CheckUserExists(gomock.Eq(test.userData)).Times(1)
			mock.EXPECT().CreateNewUser(gomock.Eq(test.userData)).Times(1).Return(nil)
		}
		handler := http.HandlerFunc(Registration)
		recorder := httptest.NewRecorder()
		url := "/register"
		body, err := json.Marshal(ReqData)
		require.NoError(t, err)
		if test.wrongRequest {
			body, err = json.Marshal(ReqData.Login)
			require.NoError(t, err)
		}
		reader := bytes.NewReader(body)
		req, err := http.NewRequest(http.MethodPost, url, reader)
		require.NoError(t, err)
		handler.ServeHTTP(recorder, req)
		require.Equal(t, test.expStatus, recorder.Code)
	}
}
