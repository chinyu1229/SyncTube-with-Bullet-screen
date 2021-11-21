# SyncTube-with-Bullet-screen

## About The Project
使用Golang開發後端環境，主要功能為：
1. YouTube畫面同步顯示（時間軸同步）
2. 發送留言並且用彈幕顯示在YT畫面上

### Built With
* BackEnd : Golang
* FrontEnd: HTML
* API : YouTube - IFrame Player API
  * https://developers.google.com/youtube/iframe_api_reference


## Getting Started

### Prerequisites
Golang import
```
"github.com/gorilla/websocket"
"github.com/satori/go.uuid"
"github.com/gorilla/mux"
```

### Execute
```shell
go build websocketHandler.go
```
website:
```
http://localhost:8080
```

## Introduction

### Home Page
輸入YouTube影片連結，會跳轉到影片頁面
<img src="md_static/index.png">

### Sync page
#### main
若網址輸入正確，會產生一組唯一的code，讓其他人進入到此網址，同步收看輸入的YouTube影片
<img src="md_static/socket.png">

#### Bullet screen
使用YT下方白框輸入聊天內容，會以彈幕方式出現在影片畫面上

<img src="md_static/bullet.png">


#### GIF 示範
