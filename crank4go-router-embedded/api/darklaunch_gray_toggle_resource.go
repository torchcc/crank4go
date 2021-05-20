package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/darklaunch_manager"
)

const grayToggleResourceBasePath string = "/dark-launch/gray"

type DarkLaunchGrayToggleResource struct {
	basePath          string
	darkLaunchManager *darklaunch_manager.DarkLaunchManager
	*Filter
}

func NewDarkLaunchGrayToggleResource(darkLaunchManager *darklaunch_manager.DarkLaunchManager) *DarkLaunchGrayToggleResource {
	return &DarkLaunchGrayToggleResource{
		darkLaunchManager: darkLaunchManager,
		basePath:          grayToggleResourceBasePath,
		Filter:            &Filter{},
	}
}

func (t *DarkLaunchGrayToggleResource) GetDetail(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespTextPlainOk(w, fmt.Sprintf("DarkMode=%v, darkModeGrayTestToggle=%v",
		t.darkLaunchManager.IsDarkModeOn(), darklaunch_manager.IsGrayTestingOn()))
	return true
}

// PAth("/on")
func (t *DarkLaunchGrayToggleResource) PutOn(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	if !t.darkLaunchManager.IsDarkModeOn() {
		RespTextPlainWithStatus(w, "Forbidden request, ErrorID="+uuid.New().String(), http.StatusForbidden)
	} else {
		darklaunch_manager.TurnGrayTestingOn("turn on gray testing by rest call")
		RespTextPlainOk(w, fmt.Sprintf("DarkMode=%v, darkModeGrayTestToggle=%v", t.darkLaunchManager.IsDarkModeOn(), darklaunch_manager.IsGrayTestingOn()))
	}
	return true
}

// PAth("/off")
func (t *DarkLaunchGrayToggleResource) PutOff(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	if !t.darkLaunchManager.IsDarkModeOn() {
		RespTextPlainWithStatus(w, "Forbidden request, ErrorID="+uuid.New().String(), http.StatusForbidden)
	} else {
		darklaunch_manager.TurnGrayTestingOff("turn off gray testing by rest call")
		RespTextPlainOk(w, fmt.Sprintf("DarkMode=%v, darkModeGrayTestToggle=%v", t.darkLaunchManager.IsDarkModeOn(), darklaunch_manager.IsGrayTestingOn()))
	}
	return true
}

func (t *DarkLaunchGrayToggleResource) RegisterResourceToHttpRouter(httpRouter *httprouter.Router, rootPath string) {
	basePath := rootPath + t.basePath
	httpRouter.GET(basePath, t.convertToHttpRouterHandlerWithFilters(t.GetDetail))
	httpRouter.PUT(basePath+"/on", t.convertToHttpRouterHandlerWithFilters(t.PutOn))
	httpRouter.PUT(basePath+"/off", t.convertToHttpRouterHandlerWithFilters(t.PutOff))
}
