# Messaging API (Go + Postgres)

API segura para app de mensagens com criptografia ponta‑a‑ponta por design (o servidor armazena somente ciphertext). Inclui registro com convite, senha + PIN, avatares, envio/edição/remoção de mensagens, respostas, anexos (imagens/vídeos), último ativo e limpeza de chat.

## Stack
- Go (Gin)
- Postgres 16
- JWT (HS256)
- Argon2id para senha e PIN
- Armazenamento em disco para avatares e anexos (`/data` volume)

## Rodando (Ubuntu 22.04 + Cosmos)
1. Crie um arquivo `.env` (opcional) com:
   - `JWT_SECRET=troque-isto`
2. Build e subir:
```bash
docker compose build --no-cache && docker compose up -d
```
3. A porta exposta será aleatória. Descubra com:
```bash
docker compose ps
```
Procure a coluna "Ports" (ex.: `0.0.0.0:49123->8081/tcp`). Use a porta publicada (49123 no exemplo).

4. Dados persistentes:
   - Postgres: volume `db_data`
   - Avatares/Uploads: volume `app_data` montado em `/data`

5. Admin pré-criado:
   - usuário: `admin`
   - senha: `admin`
   - PIN: `0000`
   - invite code: `DEFAULT-INVITE-0001`

## E2E por design
- O cliente gera e guarda chaves privadas. O servidor recebe/apenas armazena `ciphertext` e metadados (ex.: `nonce`).
- Para anexos, recomenda-se criptografar o arquivo no cliente antes do upload.

## Endpoints principais
- POST `/api/v1/auth/register` {inviteCode, username, displayName, password, pin, publicKey}
- POST `/api/v1/auth/login` {username, password, pin}
- GET `/api/v1/users/me` (Bearer)
- PUT `/api/v1/users/me/password` {oldPassword, oldPin, newPassword, newPin} (Bearer)
- POST `/api/v1/users/me/avatar` multipart form `avatar` (Bearer)
- POST `/api/v1/chats` {title?, isGroup, memberIds[]} (Bearer)
- DELETE `/api/v1/chats/:id/clear` (Bearer)
- POST `/api/v1/messages` multipart form com fields `chatId,ciphertext,nonce,replyToId?` e `files[]` (Bearer)
- PATCH `/api/v1/messages/:id` {ciphertext, nonce} (Bearer)
- DELETE `/api/v1/messages/:id` (Bearer)
- GET `/api/v1/media/avatar` (Bearer)
- GET `/api/v1/media/attachments/:id` (Bearer)

## Exemplo de uso no app C# (.NET)

### Login
```csharp
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;

var baseUrl = "http://SEU_HOST:PORTA"; // Porta aleatória obtida via `docker compose ps`
var client = new HttpClient { BaseAddress = new Uri(baseUrl) };

var loginBody = new { username = "admin", password = "admin", pin = "0000" };
var loginRes = await client.PostAsync("/api/v1/auth/login", new StringContent(JsonSerializer.Serialize(loginBody), Encoding.UTF8, "application/json"));
loginRes.EnsureSuccessStatusCode();
var loginJson = await loginRes.Content.ReadAsStringAsync();
using var doc = JsonDocument.Parse(loginJson);
var token = doc.RootElement.GetProperty("token").GetString();
client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
```

### Criar chat e enviar mensagem (E2E: ciphertext vindo do cliente)
```csharp
// criar chat
var createChatBody = new { isGroup = false, memberIds = new [] { "<outro-user-id>" } };
var chatRes = await client.PostAsync("/api/v1/chats", new StringContent(JsonSerializer.Serialize(createChatBody), Encoding.UTF8, "application/json"));
chatRes.EnsureSuccessStatusCode();
var chatJson = await chatRes.Content.ReadAsStringAsync();
var chatId = JsonDocument.Parse(chatJson).RootElement.GetProperty("chatId").GetString();

// enviar mensagem com anexo
var form = new MultipartFormDataContent();
form.Add(new StringContent(chatId!), "chatId");
form.Add(new StringContent("<ciphertext-base64>"), "ciphertext");
form.Add(new StringContent("<nonce-base64>"), "nonce");
// opcional: anexos
var fileContent = new ByteArrayContent(File.ReadAllBytes("/caminho/arquivo.png"));
fileContent.Headers.ContentType = new MediaTypeHeaderValue("image/png");
form.Add(fileContent, "files", "arquivo.png");
var sendRes = await client.PostAsync("/api/v1/messages", form);
sendRes.EnsureSuccessStatusCode();
```

### Trocar senha e PIN
```csharp
var body = new { oldPassword = "admin", oldPin = "0000", newPassword = "novaSenha123", newPin = "1234" };
var res = await client.PutAsync("/api/v1/users/me/password", new StringContent(JsonSerializer.Serialize(body), Encoding.UTF8, "application/json"));
res.EnsureSuccessStatusCode();
```

## Segurança
- Senha e PIN com Argon2id.
- JWT expira em 24h.
- Autorização por chat para baixar anexos.
- CORS restrito a necessidades básicas. Coloque `ENABLE_TLS=true` e monte `/data/tls/server.crt` e `/data/tls/server.key` para ativar HTTPS no container (também é possível terminar TLS no Cosmos).

## Notas
- Portas: o container escuta internamente 8081 (HTTP). O compose publica porta aleatória no host.
- Invite code: apenas quem possuir um código válido consegue registrar.
- O servidor não tem acesso ao plaintext das mensagens; o app C# deve cifrar e decifrar no cliente.