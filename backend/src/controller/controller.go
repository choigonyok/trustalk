package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/choigonyok/couple-chat-service/src/model"
	"github.com/gorilla/websocket"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 모든 클라이언트와 서버 간의 connection을 저장하는 map. KEY = uuid, VALUE = conn
var conns = make(map[string]*websocket.Conn)

// 커넥션을 끊으면 작동하는 timer를 저장하는 map. KEY = connection_id, VALUE = timer
var timerMap  = make(map[int]*time.Timer)

var mutex = &sync.Mutex{}

func ConnectDB(driverName, dbData string) {
	err := model.OpenDB(driverName, dbData)
	if err != nil {
		fmt.Println("ERROR #73 : ", err.Error())
	}
}

func UnConnectDB() {
	err := model.CloseDB()
	if err != nil {
		fmt.Println("ERROR #74 : ", err.Error())
	}
}

// 서버 시간대를 클라이언트/DB와 일치시키기 위해 location 설정
func getTimeNow() time.Time {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		fmt.Println("ERROR #91 : ", err.Error())
	}
	now := time.Now()
	t := now.In(loc)
	return t
}

// ID, Password 유효성 검사
func checkIDandPWCorrect(ID string, PW string) bool {
	isIDCorrect, _ := regexp.MatchString("^[a-z][a-z0-9]+$",ID)
	isPWCorrect, _ := regexp.MatchString("^[a-z0-9]*$", PW)
	if len(ID) >= 21 {
		return false
	} else if len(PW) >= 21 {
		return false
	} else if !isIDCorrect {
		return false
	} else if !isPWCorrect {
		return false
	} else {
		return true
	}
}

// cookie의 uuid를 이용해 usr의 connection id를 리턴
func GetConnIDByCookie(c *gin.Context) (int, error) {
	uuid, err1 := model.CookieExist(c)
	if err1 != nil {
		return 0, err1
	}

	conn_id, err2 := model.SelectConnIDByUUID(uuid)
	if err2 != nil {
		return 0, err2
	}
	return conn_id, nil
}

// 회원가입	
func SignUpHandler(c *gin.Context) {
	signUpData := model.UsrsData{}
	err := c.ShouldBindJSON(&signUpData)
	if err != nil {
		fmt.Println("ERROR #6 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !checkIDandPWCorrect(signUpData.ID, signUpData.Password) {
		c.String(http.StatusBadRequest, "%v", "ID와 PW의 최대 길이는 20자로 제한됩니다. 또한 영어 소문자로 시작하는 영어소문자와 숫자의 조합만 유효합니다.")
		return
	}
	
	signUpData.UUID = uuid.New().String()

	err = model.InsertUsr(signUpData.ID, signUpData.Password, signUpData.UUID)
	if err != nil {
		fmt.Println("ERROR #9 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Writer.WriteHeader(http.StatusOK)
}

// 회원가입 시 아이디 중복체크
func IDCheckHandler(c *gin.Context){
	input := struct {
		ID string `json:"input_id"`
	}{}
	
	err := c.ShouldBindJSON(&input)
	if err != nil {
		fmt.Println("ERROR #4 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	isExist, err := model.CheckUsrByID(input.ID)
	if err != nil {
		fmt.Println("ERROR #5 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isExist == true {
		c.Writer.WriteHeader(http.StatusBadRequest)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}
}

// 로그인
func LogInHandler(c *gin.Context){
	logInData := model.UsrsData{}
	err := c.ShouldBindJSON(&logInData)
	if err != nil {
		fmt.Println("ERROR #10 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	uuid, err := model.GetUUIDByIDandPW(logInData.ID, logInData.Password)
	if err != nil {
		fmt.Println("ERROR #11 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	if uuid != "" {
		c.SetCookie("uuid", uuid, 60*60, "/", os.Getenv("ORIGIN"),false,true)
		c.Writer.WriteHeader(http.StatusOK)
	} else {
		c.Writer.WriteHeader(http.StatusBadRequest)
	}
}

// 비밀번호 변경
func ChangePasswordHandler(c *gin.Context) {
	uuid, err1 := model.CookieExist(c)
	if err1 != nil {
		fmt.Println("ERROR #93 : ", err1.Error())
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	pwData := model.UsrsData{}
	
	err2 := c.ShouldBindJSON(&pwData)
	if err2 != nil {
		fmt.Println("ERROR #93 : ", err2.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	isPWCorrect, err3 := regexp.MatchString("^[a-z0-9]*$", pwData.Password)
	if err3 != nil {
		fmt.Println("ERROR #94 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isPWCorrect {
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}

	err4 := model.ChangePassword(pwData.Password, uuid)
	if err4 != nil {
		fmt.Println("ERROR #94 : ", err4.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.Writer.WriteHeader(http.StatusOK)
}

// 로그아웃
func LogOutHandler(c *gin.Context){
	uuid, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	c.SetCookie("uuid", uuid, -1, "/", os.Getenv("ORIGIN"), false, true)
	c.Writer.WriteHeader(http.StatusOK)
}

// 기존 로그인 되있던 상태인지 쿠키 확인	
func AlreadyLogInCheckHandler(c *gin.Context){
	conn_id, err := GetConnIDByCookie(c)

	if err != nil {
		fmt.Println("ERROR #121 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return 
	}

	if conn_id == 0 {
		c.String(http.StatusOK, "%v", "NOT_CONNECTED")
	} else {
		c.String(http.StatusOK, "%v", "CONNECTED")
	}
}

// 회원탈퇴
func WithDrawalHandler(c *gin.Context){
	uuid, err1 := model.CookieExist(c)
	if err1 != nil {
		fmt.Println("ERROR #82 : ", err1.Error())
	}

	conn_id, err3 := model.SelectConnIDByUUID(uuid)
	if err3 != nil {
		fmt.Println("ERROR #84 : ", err3.Error())
	}
	
	if conn_id == 0 {
		err2 := model.DeleteUsrByUUID(uuid)
		if err2 != nil {
			fmt.Println("ERROR #83 : ", err2.Error())
		}
		c.SetCookie("uuid", "", -1, "/", os.Getenv("ORIGIN"), false, true)
		c.Writer.WriteHeader(http.StatusOK)
		return
	} else {
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}
}

// 커넥션 끊기
func CutConnectionHandler(c *gin.Context){
	uuid, err1 := model.CookieExist(c)
	if err1 != nil {
		fmt.Println("ERROR #85 : ", err1.Error())
	}

	conn_id, err2 := model.SelectConnIDByUUID(uuid)
	if err2 != nil {
		fmt.Println("ERROR #86 : ", err2.Error())
	}

	if timerMap[conn_id] != nil {
		c.String(http.StatusBadRequest, "%v", "ALREADY_REGISTER")
	} else {
		setConnDeleteTimer(uuid, conn_id)
		c.Writer.WriteHeader(http.StatusOK)
	}
}

// 커넥션 7일 후 종료를 위한 타이머 설정 (TEST를 위해 2분으로 설정)
func setConnDeleteTimer(uuid string, connection_id int) {
	setTime := getTimeNow().Add(2 * time.Minute)
	timer := time.NewTimer(setTime.Sub(getTimeNow()))
	timerMap[connection_id] = timer
	go func() {
		<-timer.C
		first_usr, second_usr, conn_id, err1 := model.GetConnectionByUsrsUUID(uuid)
		if err1 != nil {
			fmt.Println("ERROR #129 : ", err1.Error())
		}

		// 서버에 저장되어있던 채팅 파일 삭제
		chatDatas, _ := model.SelectChatByUsrsUUID(first_usr, second_usr)
		for _, v := range chatDatas {
			if v.Is_file == 1 {
				err := filepath.Walk("assets", func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						if strings.Contains(info.Name(), strconv.Itoa(v.Chat_id)+"-") {
							err := os.Remove(path)
							if err != nil {
								return err
							}
						}
					}
					return nil
				})
				if err != nil {
					fmt.Println("ERROR #140 : ", err.Error())
				}
			}
		}

		// connection 관련 db 레코드 삭제
		err2 := model.DeleteConnectionByConnID(first_usr, second_usr, conn_id)
		if err2 != nil {
			fmt.Println("ERROR #90 : ", err2.Error())
		}
		timerMap[connection_id] = nil
	}()
}

// 커넥션 끊기 취소
func RollBackConnectionHandler(c *gin.Context) {
	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #122 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	if timerMap[conn_id] == nil {
		c.Writer.WriteHeader(http.StatusNoContent)
		return
	} else {
		timer := timerMap[conn_id]
		go func(){
			timer.Stop()
			<-timer.C
		}()
		timerMap[conn_id] = nil
		c.Writer.WriteHeader(http.StatusOK)
	}
}

// 커넥션 연결 요청
func ConnRequestHandler(c *gin.Context){
	uuid, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	isExist, err := model.CheckRequestByRequesterUUID(uuid)
	if err != nil {
		fmt.Println("ERROR #19 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	if isExist {
		c.String(http.StatusBadRequest, "%v", "ALREADY_REQUEST")
		return
	}

	id, err := model.SelectIDFromUsrsByUUID(uuid)
	if err != nil {
		fmt.Println("ERROR #78 : ", err.Error())
	}

	input := struct {
		ID string `json:"input_id"`
	}{}
	err = c.ShouldBindJSON(&input)
	if err != nil {
		fmt.Println("ERROR #20 : ", err.Error())
	}
	// 입력한 ID에 맞는 사용자 DATA DB에서 불러오기

	isExist, targetConnID, targetUUID, err := model.SelectConnIDandUUIDFromUsrsByID(input.ID)
	if err != nil {
		fmt.Println("ERROR #21 : ", err.Error())
	}
	if isExist {
		if targetUUID == uuid {
			c.String(http.StatusBadRequest, "%v", "NOT_YOURSELF")
		} else if targetConnID != 0 {
			c.String(http.StatusBadRequest, "%v", "ALREADY_CONNECTED")
		} else {
			// 요청된 정보를 DB에 저장
			err = model.InsertRequest(uuid, targetUUID, time.Now().Format("01/02 15:04"), id, input.ID)
			if err != nil {
				fmt.Println("ERROR #22 : ", err.Error())
			}
			c.Writer.WriteHeader(http.StatusOK)
		}
	} else {
	// ID가 존재하지 않는 ID면
		c.String(http.StatusBadRequest, "%v", "NOT_EXIST")
	}
}

// 요청받은 request 목록 가져오기
func GetRecieveRequestHandler(c *gin.Context){
	uuid, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	requestedDatas, err := model.SelectRecieveRequestByTargetUUID(uuid)
	if err != nil {
		fmt.Println("ERROR #13 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	marshaledRequestedData, err := json.Marshal(requestedDatas)
	if err != nil {
		fmt.Println("ERROR #15 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Writer.Write(marshaledRequestedData)
}


// 요청한 request 목록 가져오기
func GetSendRequestHandler(c *gin.Context){
	uuid, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	requestingData, err := model.SelectSendRequestByTargetUUID(uuid)
	if err != nil {
		fmt.Println("ERROR #16 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	marshaledRequestingData, err := json.Marshal(requestingData)
	if err != nil {
		fmt.Println("ERROR #18 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	c.Writer.Write(marshaledRequestingData)
}

// 커넥션 연결 후, DB의 자신과 상대 관련 요청 전체 삭제 + conn_id 생성
func DeleteRestRequestHandler(c *gin.Context){
	myUUID, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	
	target := struct {
		UUID string `json:"uuid_delete"`
	}{}

	err1 := c.ShouldBindJSON(&target)
	if err1 != nil {
		fmt.Println("ERROR #23 : ", err1.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	err2 := model.InsertConnection(target.UUID, myUUID, time.Now().Format("2006/01/02"))
	if err2 != nil {
		fmt.Println("ERROR #24 : ", err2.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	connID, err3 := model.SelectConnectionIDByUsrsUUID(target.UUID, myUUID)
	if err3 != nil {
		fmt.Println("ERROR #25 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	err4 := model.UpdateUsrsConnID(connID, target.UUID)
	if err4 != nil {
		fmt.Println("ERROR #26 : ", err4.Error())
		return
	}

	err5 := model.UpdateUsrsOrder(connID, myUUID)
	if err5 != nil {
		fmt.Println("ERROR #27 : ", err5.Error())
		return
	}

	err6 := model.DeleteRestRequest(target.UUID, myUUID)
	if err6 != nil {
		fmt.Println("ERROR #28 : ", err6.Error())
		return
	}

	err7 := model.InsertBeAboutToDelete(connID)
	if err7 != nil {
		fmt.Println("ERROR #24 : ", err7.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// 받은 요청 중 선택해서 요청을 삭제
func DeleteOneRequestHandler(c *gin.Context){
	request_id := c.Param("param")

	err := model.DeleteRequestByRequestID(request_id)
	if err != nil {
		fmt.Println("ERROR #29 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
	}
}

// answers 불러오기
func GetAnswerHandler(c *gin.Context){
	uuid, err := model.CookieExist(c)

	conn_id, err := model.SelectConnIDByUUID(uuid)
	if err != nil {
		fmt.Println("ERROR #123 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	order, err := model.GetUsrOrderByUUID(uuid)

	answerDatas, err := model.GetAnswerandQuestionContentsByConnIDWithOrder(conn_id, order)
	if err != nil {
		fmt.Println("ERROR #31 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	mashaledAnswerData, err := json.Marshal(answerDatas)
	if err != nil {
		fmt.Println("ERROR #33 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	c.Writer.Write(mashaledAnswerData)
}

// Websocket 프로토콜로 업그레이드 및 메시지 read/write
func UpgradeHandler(c *gin.Context){
	
	uuid, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	var upgrader  = websocket.Upgrader{
		WriteBufferSize: 1024,
		ReadBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == os.Getenv("ORIGIN")
		    },
	}

	conn, err1 := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err1 != nil {
		fmt.Println("ERROR #34 : ", err1.Error())
		return
	}
	defer conn.Close()
	defer func(){
		mutex.Lock()
		conns[uuid] = nil
		mutex.Unlock()
	}()
	
	mutex.Lock()
	conns[uuid] = conn
	// conn객체를 읽어야함
	mutex.Unlock()
	
	// 클라이언트에 uuid 전달, 그래야 클라이언트에게 채팅을 표시할 때
	// 누가 보낸 채팅인지 UUID로 구분해서 표시할 수 있음
	jsonUUID := struct {
		UUID string `json:"uuid"`
	}{
		uuid,
	}
	err2 := conn.WriteJSON(jsonUUID)
	if err2 != nil {
		fmt.Println("ERROR #35 : ", err2.Error())
		return
	}

	first_uuid, second_uuid, conn_id, err3 := model.GetConnectionByUsrsUUID(uuid)
	if err3 != nil {
		fmt.Println("ERROR #36 : ", err3.Error())
		return
	}

	initialChats, err4 := model.SelectChatByUsrsUUID(first_uuid, second_uuid)
	if err4 != nil {
		fmt.Println("ERROR #37 : ", err4.Error())
		return 
	}
	
	if len(initialChats) != 0 {
		err5 := conn.WriteJSON(initialChats)
		if err5 != nil {
			fmt.Println("ERROR #38 : ", err5.Error())
			return
		}
	}

	// 이전에 대답 안하고 커넥션 종료된 question 있는지 확인
	order, err6 := model.GetUsrOrderByUUID(uuid)
	if err6 != nil {
		fmt.Println("ERROR #55 : ", err6.Error())
		return
	}

	question_id, err7 := model.QuestionIDOfEmptyAnswerByOrder(order, conn_id)
	if err7 != nil {
		fmt.Println("ERROR #79 : ", err7.Error())
		return
	}

	if question_id != 0 {
		_, questionContents, err8 := model.GetQuestionByQuestionID(question_id)
		if err8 != nil {
			fmt.Println("ERROR #80 : ", err8.Error())
			return
		}

		questiondata := model.ChatData{
			Text_body: questionContents,
			Writer_id: "question",
			Write_time: time.Now().Format("2006/01/02 03:04"),
			Is_answer: 1,
			Is_deleted: 0,
			Is_file: 0,
			Chat_id: 0,
			Question_id: question_id,
		}
		questiondatas := []model.ChatData{}
		questiondatas = append(questiondatas, questiondata)
		err := conn.WriteJSON(questiondatas)
		if err != nil {
			fmt.Println("ERROR #56 : ", err.Error())
			return
		}
	}

	go func(){
			ticker := time.NewTicker(30 * time.Second) // 30초마다 ping 메시지 보내기
			defer ticker.Stop()
		
			for range ticker.C {
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					fmt.Println("ERROR #120 : ", err.Error())
					break
				}
			}
	}()

// 메시지를 읽고 쓰는 부분, 읽은 메시지는 DB에 저장됨
	for { 
		var chatData []model.ChatData

		err := conn.ReadJSON(&chatData)
		if err != nil {
			fmt.Println("ERROR #39 : ", err.Error())
			break;
		}

		// 일반채팅이면 chat table에 저장, question에 대한 답이면 answer table에 저장
		if chatData[0].Is_answer == 1 {
			recieveAnswer(uuid, conn_id, chatData, first_uuid)
		} else if chatData[0].Is_deleted == 1 {
			if chatData[0].Is_file == 1 {
				err := filepath.Walk("assets", func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						if strings.Contains(info.Name(), strconv.Itoa(chatData[0].Chat_id)+"-") {
							err := os.Remove(path)
							if err != nil {
								return err
							}
						}
					}
					return nil
				})
				if err != nil {
					fmt.Println("ERROR #138 : ", err.Error())
				}
			}
			err = model.DeleteChatByChatID(chatData[0].Chat_id)
			if err != nil {
				fmt.Println("ERROR #95 : ", err.Error())
			}
			
		} else if chatData[0].Is_file != 1 {
			chat_id, err := model.InsertChatAndGetChatID(chatData[0].Text_body, uuid, chatData[0].Write_time, 0, 0)
			// 어차피 커넥션 당 메시지 하나씩 전송 받으니까 slice index는 0으로 설정
			if err != nil {
				fmt.Println("ERROR #40 : ", err.Error())
			}
			chatData[0].Chat_id = chat_id
		} else {
			chatID, err := model.GetChatIDFromRecentFileChatByUUID(uuid)
			if err != nil {
				fmt.Println("ERROR #134 : ", err.Error())
			}
			chatData[0].Chat_id = chatID
			text_body, err2 := model.GetTextBodyByChatID(chatID)
			if err2 != nil {
				fmt.Println("ERROR #135 : ", err2.Error())
			}
			chatData[0].Text_body = text_body
		}
		
		target_conn := []*websocket.Conn{}

		mutex.Lock()
		if conns[first_uuid] != nil && conns[second_uuid] != nil {
			first_conn := conns[first_uuid]
			second_conn := conns[second_uuid]
			target_conn = append(target_conn, first_conn, second_conn)
		
		} else if conns[first_uuid] != nil {
			first_conn := conns[first_uuid]
			target_conn = append(target_conn, first_conn)
		
		} else {
			second_conn := conns[second_uuid]
			target_conn = append(target_conn, second_conn)	
		}
		mutex.Unlock()

		// 커넥션 연결이 안되어있으면 보내면 nil pointer 오류 생김
		// 모든 커넥션에 메시지 write
		
		if chatData[0].Is_answer != 1 {
			for _, item := range target_conn {
				err := item.WriteJSON(chatData)
				if err != nil {
					fmt.Println("ERROR #43 : ", err.Error())
				}
			}
		}
		sendQuestion(chatData, conn_id, target_conn)
	}
}

func sendQuestion(chatData []model.ChatData, conn_id int, target_conn []*websocket.Conn){
// 채팅 중 단어가 발견되면 단어 관련된 질문을 커플에게 던지는 기능
	// 1. 단어를 먼저 다 뽑아서
	var target_word, question_contents string
	var question_id int
	r, err := model.SelectQuetions()
	defer r.Close()
	if err != nil {
		fmt.Println("ERROR #44 : ", err.Error())
	}
	for r.Next() {
		// 2. 방금 READ한 채팅에 단어가 있는지 돌면서 확인
		r.Scan(&target_word, &question_id, &question_contents)	
		if strings.Contains(chatData[0].Text_body, target_word) {
			// 3. 단어가 발견되면 이전에 답을 한 전적이 있는지 검색
			isExist, err := model.CheckAnswerByConnIDandQuestionID(conn_id, question_id)
			if err != nil {
				fmt.Println("ERROR #45 : ", err.Error())
				return
			}

			// 4. 단어도 발견됐고, 이전에 했던 질문도 아니면 질문 WRITE
			if !isExist {
				questiondata := model.ChatData{
					Text_body: question_contents,
					Writer_id: "question",
					Write_time: time.Now().Format("2006/01/02 03:04"),
					Is_answer: 1,
					Is_deleted: 0,
					Is_file: 0,
					Chat_id: 0,
					Question_id: question_id,
				}
				questiondatas := []model.ChatData{}
				questiondatas = append(questiondatas, questiondata)

				for _, item := range target_conn {
					err := item.WriteJSON(questiondatas)
					if err != nil {
						fmt.Println("ERROR #46 : ", err.Error())
					}
				}
				// 5. answer에 답 적기 (는 위에 READ에서 처리)
				err = model.InsertAnswer(chatData[0].Write_time, conn_id, question_id)
				if err != nil {
					fmt.Println("ERROR #42 : ", err.Error())
				}
			}
		}
	}
}

func recieveAnswer(uuid string, conn_id int, chatData []model.ChatData, first_uuid string){
	isExist, err1 := model.CheckAnswerByConnIDandQuestionID(conn_id, chatData[0].Question_id)
	if err1 != nil {
		fmt.Println("ERROR #41 : ", err1.Error())
		return
	}
	
	if isExist {
		var err2 error
		if first_uuid == uuid {
			err2 = model.UpdateFirstAnswerByQuestionID(chatData[0].Text_body, chatData[0].Question_id)
		} else {
			err2 = model.UpdateSecondAnswerByQuestionID(chatData[0].Text_body, chatData[0].Question_id)
		}
		if err2 != nil {
			fmt.Println("ERROR #50 : ", err2.Error())
		}
	}
}

// 커넥션별로 채팅에서 가장 많이 사용된 단어 불러오기
func GetMostUsedWordsHandler(c *gin.Context){
	rankNumString := c.Param("ranknum")
	uuid, err := model.CookieExist(c)
	if err != nil {
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}
	rankNumInt, err := strconv.Atoi(rankNumString)
	if err != nil {
		fmt.Println("ERROR #59 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return	
	}
	firstUUID, secondUUID, _, err := model.GetConnectionByUsrsUUID(uuid)
	
	var ohterFrequentWords []string
	var err2 error
	if firstUUID == uuid {
		ohterFrequentWords, err2 = model.GetFrequentWords(secondUUID, rankNumInt)	
		if err2 != nil {
			fmt.Println("ERROR #80 : ", err2.Error())
		}
	} else {
		ohterFrequentWords, err2 = model.GetFrequentWords(firstUUID, rankNumInt)
		if err2 != nil {
			fmt.Println("ERROR #80 : ", err2.Error())
		}
	}
	myFrequentWords, err3 := model.GetFrequentWords(uuid, rankNumInt)
	if err3 != nil {
		fmt.Println("ERROR #80 : ", err3.Error())
	}

	if myFrequentWords == nil || ohterFrequentWords == nil {
		c.Writer.WriteHeader(http.StatusLengthRequired)
		return
	}

	sendData := struct {
		MyWords []string `json:"mywords"`
		OtherWords []string `json:"otherwords"`
	}{
		myFrequentWords,
		ohterFrequentWords,
	}

	marshaledData, err := json.Marshal(sendData)
	if err != nil {
		fmt.Println("ERROR #60 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return	
	} 

	c.Writer.Write(marshaledData)
}

// words ranking에서 제외된 단어 불러오기
func GetExceptWordsHandler(c *gin.Context){
	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #124 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}
	
	exceptWords, err2 := model.GetExceptWords(conn_id)
	if err2 != nil {
		fmt.Println("ERROR #62 : ", err2.Error())
	}
	if len(exceptWords) == 0 {
		c.Writer.WriteHeader(http.StatusNoContent)	
		return
	}
	marshaledData, err3 := json.Marshal(exceptWords)
	if err3 != nil {
		fmt.Println("ERROR #81 : ", err3.Error())
	}
	c.Writer.Write(marshaledData)
}

// words ranking에서 제외시킬 단어 추가
func InsertExceptWordHandler(c *gin.Context){
	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #125 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	Input := struct {
		Except_word string `json:"except_word"`
	}{}
	err1 := c.ShouldBindJSON(&Input)
	if err1 != nil {
		fmt.Println("ERROR #63 : ", err1.Error())
	}
	isExist, err2 := model.CheckWordAlreadyExcepted(conn_id, Input.Except_word)
	if err2 != nil {
		fmt.Println("ERROR #63 : ", err2.Error())
	}
	if isExist {
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	} else {
		err2 := model.InsertExceptWord(conn_id, Input.Except_word)
		if err2 != nil {
		fmt.Println("ERROR #63 : ", err2.Error())
		}
		c.Writer.WriteHeader(http.StatusOK)
	}
}

// words ranking에서 제외시켰던 단어 취소
func DeleteExceptWordHandler(c *gin.Context){
	cancleWord := c.Param("param")

	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #126 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	err3 := model.CancleExceptWord(conn_id, cancleWord)
	if err3 != nil {
		fmt.Println("ERROR #68 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	c.Writer.WriteHeader(http.StatusOK)
}

// input된 날짜에 작성된 채팅 리턴
func GetChatDateHandler(c *gin.Context) {
	myUUID, err1 := model.CookieExist(c)
	if err1 != nil {
		fmt.Println("ERROR #110 : ", err1.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	first_uuid, second_uuid, _, err2 := model.GetConnectionByUsrsUUID(myUUID)
	if err2 != nil {
		fmt.Println("ERROR #111 : ", err2.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	year := c.Query("year")
	month := c.Query("month")
	date := c.Query("date")
	monthNum, _  := strconv.Atoi(month)
	dateNum, _  := strconv.Atoi(date)
	if monthNum < 10 {
		month = "0"+month
	}
	if dateNum < 10 {
		date = "0"+date
	}

	chats, err3 := model.SelectChatByUsrsUUID(first_uuid, second_uuid)
	if err3 != nil {
		fmt.Println("ERROR #112 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	var sendData []model.ChatData

	for i := 0; i < len(chats); i++ {
		if strings.Contains(chats[i].Write_time, year+"-"+month+"-"+date) {
			sendData = append(sendData, chats[i])
			break;
		}
	}

	if len(sendData) == 0 {
		c.Writer.WriteHeader(http.StatusNoContent)
		return
	}

	marshaledData, err4 := json.Marshal(sendData)
	if err4 != nil {
		fmt.Println("ERROR #113 : ", err4.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Writer.Write(marshaledData)
}

// 검색한 단어가 포함된 채팅 리턴
func GetChatWordHandler(c *gin.Context) {
	targetWord := c.Param("param")

	uuid, err1 := model.CookieExist(c)
	if err1 != nil {
		fmt.Println("ERROR #97 : ", err1.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	first_uuid, second_uuid, _, err2 := model.GetConnectionByUsrsUUID(uuid)
	if err2 != nil {
		fmt.Println("ERROR #98 : ", err2.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	chats, err3 := model.SelectChatByUsrsUUID(first_uuid, second_uuid)
	if err3 != nil {
		fmt.Println("ERROR #99 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	var SearchChatSlice []model.ChatData

	for i := 0; i < len(chats); i++ {
		if strings.Contains(chats[i].Text_body, targetWord) {
			SearchChatSlice = append(SearchChatSlice, chats[i])
		}
	}

	if len(SearchChatSlice) == 0 {
		c.Writer.WriteHeader(http.StatusNotFound)
		return
	}

	marshaledData, err4 := json.Marshal(SearchChatSlice)
	if err4 != nil {
		fmt.Println("ERROR #100 : ", err4.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Writer.Write(marshaledData)
}

// 캘린터에 일정 추가 + d-day로 지정된 일정이면 기존 d-day를 수정
func InsertAnniversaryHandler(c *gin.Context) {
	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #127 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	var anniversaryData model.AnniversaryData
	
	err2 := c.ShouldBindJSON(&anniversaryData)
	if err2 != nil {
		fmt.Println("ERROR #102 : ", err2.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	anniversaryData.Connection_id = conn_id

	anniversary_id, err4 := model.GetDDayAnniversaryIDByConnID(conn_id)
	if err4 != nil {
		fmt.Println("ERROR #104 : ", err4.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if anniversary_id != 0 && anniversaryData.D_day == true {
		err5 := model.ChangeDDayZeroByAnniversaryID(anniversary_id)
		if err5 != nil {
			fmt.Println("ERROR #105 : ", err5.Error())
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}	
	}

	err6 := model.InsertAnniversaryByConnID(anniversaryData)
	if err6 != nil {
		fmt.Println("ERROR #106 : ", err6.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// 저장된 캘린더 일정 중 해당 연도/달에 맞는 일정들 불러오기
func GetAnniversaryHandler(c *gin.Context){
	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #128 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	month := c.Query("month")
	year := c.Query("year")

	anniversaryDatas, err3 := model.GetAnniversaryByConnIDAndMonthAndYear(conn_id, month, year)
	if err3 != nil {
		fmt.Println("ERROR #107 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(anniversaryDatas) == 0 {
		c.Writer.WriteHeader(http.StatusNoContent)
		return
	}

	marshaledData,  err4 := json.Marshal(anniversaryDatas)
	if err4 != nil {
		fmt.Println("ERROR #108 : ", err4.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write(marshaledData)
}

// 캘린더에서 일정 삭제
func DeleteAnniversaryHandler(c *gin.Context) {
	anniversary_id := c.Param("id")
	err := model.DeleteAnniversaryByAnniversaryID(anniversary_id)
	if err != nil {
		fmt.Println("ERROR #109 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}

}

// d-day로 지정된 일정 불러오기
func GetDDayHandler(c *gin.Context) {
	conn_id, err := GetConnIDByCookie(c)
	if err != nil {
		fmt.Println("ERROR #129 : ",  err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return 
	}

	anniversaryData, err3 := model.GetDDayByConnID(conn_id)
	if err3 != nil {
		fmt.Println("ERROR #112 : ", err3.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
			return
	}

	if len(anniversaryData) == 0  {
		c.Writer.WriteHeader(http.StatusNoContent)
	} else {
		marshaledData, err4 := json.Marshal(anniversaryData)
		if err4 != nil {
			fmt.Println("ERROR #113 : ", err4.Error())
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		c.Writer.Write(marshaledData)
	}
}

func InsertFileHandler(c *gin.Context) {
	uuid, err1 := model.CookieExist(c)
	if err1 != nil {
		fmt.Println("ERROR #130 : ", err1.Error())
	}

	f, err4 := c.FormFile("file")
	if err4 != nil {
		fmt.Println("ERROR #132 : ", err4.Error())
	}

	mimeType := f.Header.Get("Content-Type")

	if  !strings.Contains(mimeType,"application/") && !strings.Contains(mimeType,"text/") && !strings.Contains(mimeType,"audio/") && !strings.Contains(mimeType,"video/") && !strings.Contains(mimeType,"image/"){
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}

	var err3 error
	var chatID int
	if strings.Contains(mimeType,"image/") {
		chatID, err3 = model.InsertChatAndGetChatID(f.Filename, uuid, getTimeNow().Format("2006-01-02 03:04:05"), 1, 1)
	} else {
		chatID, err3 = model.InsertChatAndGetChatID(f.Filename, uuid, getTimeNow().Format("2006-01-02 03:04:05"), 1, 0)
	}
	if err3 != nil {
		fmt.Println("ERROR #132 : ", err3.Error())
	}

	err5 := c.SaveUploadedFile(f, "assets/"+strconv.Itoa(chatID)+"-"+f.Filename)
	if err5 != nil {
		fmt.Println("ERROR #133 : ", err5.Error())
	}
}

func GetFileHandler(c *gin.Context) {
	chatID := c.Param("chatID")

	var file *os.File
	err := filepath.Walk("assets", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if strings.Contains(info.Name(), chatID+"-") {
				file, err = os.Open(path)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("ERROR #137 : ", err.Error())
	}
	defer file.Close()

	_, err2 := io.Copy(c.Writer, file)
	if err2 != nil {
		fmt.Println("ERROR #132 : ", err2.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func GetFileNameHandler(c *gin.Context) {
	chatID := c.Param("chatID")

	err := filepath.Walk("assets", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if strings.Contains(info.Name(), chatID+"-") {
				fileName := strings.TrimPrefix(info.Name(),chatID+"-")
				sendData := struct {
					FileName string `json:"filename"`
				}{
					FileName: fileName,
				}
				marshaledData, err := json.Marshal(sendData)
				if err != nil {
					return err
				}

				c.Writer.Write(marshaledData)
				return nil
			}
		}
		return err
	})
	if err != nil {
		fmt.Println("ERROR #137 : ", err.Error())
		c.Writer.WriteHeader(http.StatusInternalServerError)
	}
}