// Package docs menyajikan dokumentasi API interaktif (Swagger UI) dan
// spesifikasi OpenAPI. Hanya didaftarkan bila docs.enabled true; validasi
// config mewajibkannya mati di production.
package docs

import (
	"github.com/gofiber/fiber/v3"
)

// swaggerUIVersion dipatok supaya halaman dokumentasi tidak berubah diam-diam
// ketika versi baru swagger-ui terbit di CDN.
const swaggerUIVersion = "5.17.14"

// Hash Subresource Integrity (SRI) untuk aset di atas. Browser menolak
// menjalankan file dari CDN bila isinya tidak cocok dengan hash ini,
// sehingga CDN yang disusupi tidak bisa menyisipkan kode. Perbarui hash
// setiap kali swaggerUIVersion diganti.
const (
	swaggerCSSIntegrity = "sha384-wxLW6kwyHktdDGr6Pv1zgm/VGJh99lfUbzSn6HNHBENZlCN7W602k9VkGdxuFvPn"
	swaggerJSIntegrity  = "sha384-wmyclcVGX/WhUkdkATwhaK1X1JtiNrr2EoYJ+diV3vj4v6OC5yCeSu+yW13SYJep"
)

// contentSecurityPolicy hanya mengizinkan aset Swagger UI dari CDN jsdelivr.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
	"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
	"img-src 'self' data: https://cdn.jsdelivr.net; " +
	"connect-src 'self'"

// page adalah halaman HTML Swagger UI yang memuat /openapi.yaml.
const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui.css"
        integrity="` + swaggerCSSIntegrity + `" crossorigin="anonymous">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui-bundle.js"
          integrity="` + swaggerJSIntegrity + `" crossorigin="anonymous"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui", persistAuthorization: true });
  </script>
</body>
</html>`

// Register mendaftarkan GET /openapi.yaml dan GET /docs.
func Register(app *fiber.App, spec []byte) {
	app.Get("/openapi.yaml", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml")
		return c.Send(spec)
	})
	app.Get("/docs", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		c.Set(fiber.HeaderContentSecurityPolicy, contentSecurityPolicy)
		return c.SendString(page)
	})
}
