// Package utils 提供跨包共享是实用功能
package utils

import "fmt"

func GenTipText(text string, gotourl string, urlname string) []byte {
	return []byte(fmt.Sprintf(`
	<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>无记名投票</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            background-color: #f0f0f0;
            margin: 0;
            padding: 0;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
        }
        .container {
            background-color: #fff;
            padding: 20px;
            box-shadow: 0 0 10px rgba(0,0,0,0.1);
            text-align: center;
        }
        a {
            color: #007BFF;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
<div class="container">
    <p>提示：%s</p>
    <a href="%s">%s</a>
</div>
</body>
</html>
	`, text, gotourl, urlname))
}
