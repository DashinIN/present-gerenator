# FunGreet — Авторизация через httpOnly cookies

## 1. Базовая идея

Используем две пары cookies:

| Cookie | Что хранит | TTL | Назначение |
|--------|-----------|-----|-----------|
| `access_token` | JWT access token | 15 минут | Авторизация каждого API-запроса |
| `refresh_token` | JWT refresh token | 30 дней | Обновление access_token без перелогина |

**Атрибуты cookies:**
```
HttpOnly      — JS не может прочитать (защита от XSS)
Secure        — только HTTPS
SameSite=None — работает в кросс-сайт контексте (для Telegram Mini App)
Partitioned   — изоляция по embedder origin (CHIPS, для будущей совместимости)
Domain=.fungreet.app
Path=/
```

---

## 2. Почему именно такие атрибуты — важные нюансы

### 2.1. Зачем SameSite=None

Telegram Mini App загружается внутри WebView, который браузер видит как **кросс-сайт** контекст (embedder = web.telegram.org, а твой сайт = fungreet.app). Cookies с `SameSite=Strict` или `SameSite=Lax` в этой ситуации **не будут отправлены**.

`SameSite=None` разрешает отправку cookies в любых запросах — включая те, что инициированы из Mini App. Но важно: `SameSite=None` требует `Secure` (только HTTPS).

### 2.2. Partitioned (CHIPS)

Browsers постепенно блокируют third-party cookies. Атрибут `Partitioned` создаёт отдельную "корзину" cookies для каждого embedder'а. Для Telegram Mini App это значит: cookies, выставленные когда юзер открыл Mini App из Telegram, будут сохраняться и работать стабильно, даже если браузер блокирует обычные third-party cookies.

### 2.3. Почему разделяем access и refresh

- **access_token** короткоживущий (15 мин) — утечка через уязвимый endpoint не даёт долговременного доступа
- **refresh_token** долгоживущий (30 дней), но хранится на отдельном endpoint `/api/auth/refresh` с `Path=/api/auth/refresh` — если XSS каким-то образом сможет инициировать запрос, он не получит refresh cookie при обращении к другим API

```
Set-Cookie: access_token=...;  HttpOnly; Secure; SameSite=None; Partitioned; Path=/;                    Max-Age=900
Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=None; Partitioned; Path=/api/auth/refresh;    Max-Age=2592000
```

---

## 3. CORS конфигурация (критично для cookies)

Без правильного CORS cookies просто не будут отправляться на бэкенд.

### Бэкенд (Go / Gin):

```go
r.Use(cors.New(cors.Config{
    AllowOrigins: []string{
        "https://fungreet.app",
        "https://web.telegram.org",  // для TG Web-клиента
    },
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
    AllowHeaders:     []string{"Content-Type", "Authorization"},
    ExposeHeaders:    []string{"Content-Length"},
    AllowCredentials: true,        // ← ОБЯЗАТЕЛЬНО для cookies
    MaxAge:           12 * time.Hour,
}))
```

### Фронтенд — каждый fetch должен иметь `credentials: 'include'`:

```ts
// src/lib/api.ts
export async function apiRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',  // ← отправляет cookies
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  // Автоматическое обновление токена при 401
  if (response.status === 401) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      return apiRequest(path, options);  // retry
    }
    throw new UnauthorizedError();
  }

  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }

  return response.json();
}

async function refreshAccessToken(): Promise<boolean> {
  const res = await fetch(`${API_BASE}/api/auth/refresh`, {
    method: 'POST',
    credentials: 'include',
  });
  return res.ok;
}
```

---

## 4. Поток Telegram Mini App авторизации (с cookies)

```
1. Пользователь открывает Mini App
   Фронт: detectEnvironment() → 'telegram'
   Фронт: initData = WebApp.initData (подписана ботом)

2. Фронт делает запрос авторизации:
   POST /api/auth/telegram
   Headers:
     Authorization: tma {initData}
   credentials: 'include'

3. Бэк проверяет подпись initData (HMAC-SHA256 с bot_token)
   Находит/создаёт пользователя в БД

4. Бэк отвечает:
   200 OK
   Set-Cookie: access_token=...;  HttpOnly; Secure; SameSite=None; Partitioned; Max-Age=900
   Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=None; Partitioned; Path=/api/auth/refresh; Max-Age=2592000
   Body: { "user": {...} }  ← НЕ возвращаем токен в теле, только user

5. Все последующие запросы автоматически содержат cookies
```

### Backend handler (Go):

```go
func (h *AuthHandler) Telegram(c *gin.Context) {
    authHeader := c.GetHeader("Authorization")
    if !strings.HasPrefix(authHeader, "tma ") {
        c.JSON(401, gin.H{"error": "invalid auth header"})
        return
    }
    rawInitData := strings.TrimPrefix(authHeader, "tma ")

    if err := initdata.Validate(rawInitData, h.botToken, time.Hour); err != nil {
        c.JSON(401, gin.H{"error": "invalid initData"})
        return
    }

    parsed, _ := initdata.Parse(rawInitData)
    user, _ := h.users.FindOrCreateByTelegramID(c, parsed.User)

    h.issueCookies(c, user.ID)
    c.JSON(200, gin.H{"user": user})
}

func (h *AuthHandler) issueCookies(c *gin.Context, userID int64) {
    access, _  := h.jwt.Issue(userID, "access",  15 * time.Minute)
    refresh, _ := h.jwt.Issue(userID, "refresh", 30 * 24 * time.Hour)

    // Access token — доступен на всех путях
    c.SetSameSite(http.SameSiteNoneMode)
    c.SetCookie(
        "access_token", access,
        int((15 * time.Minute).Seconds()),
        "/",
        ".fungreet.app",
        true,  // secure
        true,  // httpOnly
    )

    // Refresh token — только на /api/auth/refresh
    c.SetSameSite(http.SameSiteNoneMode)
    c.SetCookie(
        "refresh_token", refresh,
        int((30 * 24 * time.Hour).Seconds()),
        "/api/auth/refresh",
        ".fungreet.app",
        true,
        true,
    )
    
    // Gin не поддерживает Partitioned напрямую — выставляем вручную
    // Модифицируем заголовки Set-Cookie
    cookies := c.Writer.Header().Values("Set-Cookie")
    for i, cookie := range cookies {
        cookies[i] = cookie + "; Partitioned"
    }
    c.Writer.Header()["Set-Cookie"] = cookies
}
```

---

## 5. Refresh эндпоинт

```go
func (h *AuthHandler) Refresh(c *gin.Context) {
    refreshToken, err := c.Cookie("refresh_token")
    if err != nil {
        c.JSON(401, gin.H{"error": "no refresh token"})
        return
    }

    claims, err := h.jwt.Verify(refreshToken, "refresh")
    if err != nil {
        c.JSON(401, gin.H{"error": "invalid refresh token"})
        return
    }

    // Проверяем что пользователь не заблокирован
    user, err := h.users.GetByID(c, claims.UserID)
    if err != nil || user.IsBlocked {
        c.JSON(401, gin.H{"error": "user not found"})
        return
    }

    // Выпускаем НОВУЮ пару (rotation — старый refresh становится невалидным)
    h.issueCookies(c, user.ID)
    c.JSON(200, gin.H{"ok": true})
}
```

**Refresh rotation:** при каждом обновлении выпускаем и новый refresh_token тоже. Старый добавляем в "чёрный список" (Redis, TTL = 30 дней). Это защита от кражи refresh_token'а — если злоумышленник попытается использовать украденный, он получит 401, а легитимный пользователь заметит что его разлогинили.

---

## 6. OAuth 2.0 callback с cookies

После successful OAuth callback (Google/VK/Yandex) — **не передаём токен в URL** как раньше. Выставляем cookies прямо в ответе-редиректе:

```go
func (h *OAuthHandler) Callback(c *gin.Context) {
    // ... получение профиля от провайдера ...
    
    user, _ := h.users.FindOrCreateByOAuth(c, providerName, profile)
    
    h.issueCookies(c, user.ID)  // cookies выставлены
    
    // Просто редирект на главную — токены уже в cookies
    c.Redirect(302, h.frontendURL + "/")
}
```

Фронт при загрузке главной страницы делает GET `/api/user/me` — запрос автоматически отправляет cookies, бэк отдаёт профиль. Если cookies нет → редирект на `/login`.

---

## 7. Logout

```go
func (h *AuthHandler) Logout(c *gin.Context) {
    // Инвалидируем refresh_token
    if rt, err := c.Cookie("refresh_token"); err == nil {
        h.blacklist.Add(c, rt, 30*24*time.Hour)
    }

    // Стираем cookies: выставляем с Max-Age=-1
    c.SetSameSite(http.SameSiteNoneMode)
    c.SetCookie("access_token",  "", -1, "/",                    ".fungreet.app", true, true)
    c.SetCookie("refresh_token", "", -1, "/api/auth/refresh",    ".fungreet.app", true, true)

    c.JSON(200, gin.H{"ok": true})
}
```

---

## 8. Защита от CSRF

Главный минус cookie-based auth: уязвимость к CSRF-атакам. При `SameSite=None` cookies отправляются со всеми запросами, включая кросс-сайтовые.

### Решение: Double Submit Cookie + CSRF token header

1. При логине выдаём дополнительный **не-HttpOnly** cookie `csrf_token` (JS может прочитать)
2. Фронт при каждом state-changing запросе (POST/PUT/DELETE) добавляет header `X-CSRF-Token: <value>`
3. Бэк сравнивает cookie `csrf_token` и header — должны совпадать

```go
// Middleware
func CSRFMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
            c.Next()
            return
        }
        
        cookieToken, _ := c.Cookie("csrf_token")
        headerToken := c.GetHeader("X-CSRF-Token")
        
        if cookieToken == "" || headerToken == "" || cookieToken != headerToken {
            c.AbortWithStatusJSON(403, gin.H{"error": "CSRF validation failed"})
            return
        }
        
        c.Next()
    }
}
```

Фронт:
```ts
function getCsrfToken(): string {
  const match = document.cookie.match(/csrf_token=([^;]+)/);
  return match ? match[1] : '';
}

// В apiRequest добавляем:
headers: {
  'X-CSRF-Token': getCsrfToken(),
  ...options.headers,
}
```

---

## 9. Подводные камни

### 9.1. Local development
На `localhost` `Secure; SameSite=None` может не работать в некоторых браузерах. Решения:
- Использовать `https://localhost` с самоподписанным сертификатом (mkcert)
- Или в dev-режиме переключаться на `SameSite=Lax` и `Secure=false`

```go
var isDev = os.Getenv("ENV") == "development"

func (h *AuthHandler) issueCookies(c *gin.Context, userID int64) {
    sameSite := http.SameSiteNoneMode
    secure := true
    if isDev {
        sameSite = http.SameSiteLaxMode
        secure = false
    }
    // ...
}
```

### 9.2. Safari и iOS Telegram
Safari исторически строг к third-party cookies. На iOS Telegram использует `WKWebView` — поведение близко к Safari. `Partitioned` атрибут решает большинство проблем, но тестировать нужно реально на iOS устройстве перед запуском.

### 9.3. Telegram Desktop vs Mobile
- **Telegram Desktop**: встраивает Mini App через iframe на web.telegram.org
- **Telegram iOS/Android**: встраивает через нативный WebView
- Поведение cookies может отличаться — нужно тестировать на всех платформах

### 9.4. Отладка
Cookies с `HttpOnly; Secure; SameSite=None; Partitioned` сложно отлаживать:
- Chrome DevTools → Application → Cookies — видны
- `document.cookie` в консоли — HttpOnly невидимы (это фича, не баг)
- Для отладки используй Network tab → каждый запрос показывает отправленные cookies

---

## 10. Чеклист настройки

- [ ] HTTPS обязателен везде (даже в dev через mkcert)
- [ ] CORS: `AllowCredentials: true`, точный список origin (не `*`)
- [ ] Все cookies: `HttpOnly; Secure; SameSite=None; Partitioned`
- [ ] Разные `Path` для access и refresh
- [ ] Domain с точкой впереди: `.fungreet.app` (для поддоменов)
- [ ] CSRF middleware для state-changing запросов
- [ ] Refresh token rotation + blacklist в Redis
- [ ] Access token TTL 15 мин, refresh 30 дней
- [ ] Тест на Telegram Desktop, iOS, Android
- [ ] Логирование неудачных попыток refresh (подозрительная активность)
