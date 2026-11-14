package main

import (
	"embed"
	"log"
	"os"

	"github.com/janayuv/import-manager-next/backend/app/bootstrap"
	"github.com/janayuv/import-manager-next/backend/app/lifecycle"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	rt, err := bootstrap.Initialize(os.Args[1:], "")
	if err != nil {
		log.Fatalf("Import Manager failed to initialize: %v", err)
	}
	coord := lifecycle.NewCoordinator(rt)
	defer func() {
		if shutdownErr := coord.Close(); shutdownErr != nil {
			log.Printf("shutdown warning: %v", shutdownErr)
		}
	}()

	appService := NewAppService(rt)

	app := application.New(application.Options{
		Name:        rt.Config.AppName,
		Description: "Import Manager Next Generation Desktop ERP",
		Services: []application.Service{
			application.NewService(appService),
			application.NewService(rt.Security),
			application.NewService(rt.Audit),
			application.NewService(rt.Options),
			application.NewService(rt.Suppliers),
			application.NewService(rt.Items),
			application.NewService(rt.Shipments),
			application.NewService(rt.Invoices),
			application.NewService(rt.InvoiceWizard),
			application.NewService(rt.BOE),
			application.NewService(rt.BOEEntry),
			application.NewService(rt.BOESummary),
			application.NewService(rt.Expenses),
			application.NewService(rt.ExpenseReports),
			application.NewService(rt.Reports),
			application.NewService(rt.Dashboard),
			application.NewService(rt.Settings),
			application.NewService(rt.Backup),
			application.NewService(rt.GoogleDrive),
			application.NewService(rt.RecycleBin),
			application.NewService(rt.DatabaseManagement),
			application.NewService(rt.Workflow),
			application.NewService(rt.Updater),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Windows: application.WindowsOptions{
			WndProcInterceptor: nil,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Import Manager — Next Generation ERP",
		Width:            1360,
		Height:           840,
		MinWidth:         1024,
		MinHeight:        680,
		BackgroundColour: application.NewRGB(248, 250, 252),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		rt.Logger.Error("application exited with error", "error", err)
		log.Fatal(err)
	}
}
