package cmd

import (
	_ "embed"
)

//go:embed chat.js
var lemChatJS []byte

const chatHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>LEM Chat</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    html, body { height: 100%%; background: #111; }
    body {
      display: flex;
      align-items: center;
      justify-content: center;
      font-family: system-ui, -apple-system, sans-serif;
    }
    lem-chat {
      width: 720px;
      height: 85vh;
      max-height: 800px;
    }
    @media (max-width: 768px) {
      lem-chat { width: 100%%; height: 100%%; max-height: none; border-radius: 0; }
    }
  </style>
</head>
<body>
  <lem-chat
    endpoint=""
    model="%s"
    system-prompt=""
    max-tokens="%d"
  ></lem-chat>
  <script type="module" src="/chat.js"></script>
</body>
</html>`
