package api

import (
	"context"
	"fmt"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/connection"
	"github.com/HydroProtocol/hydro-scaffold-dex/backend/models"
	"github.com/HydroProtocol/hydro-sdk-backend/common"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk"
	"github.com/HydroProtocol/hydro-sdk-backend/sdk/ethereum"
	"github.com/HydroProtocol/hydro-sdk-backend/utils"
	"github.com/go-playground/validator"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"net/http"
	"os"
	"runtime"
	"time"
)

var CacheService common.IKVStore
var QueueService common.IQueue

func loadRoutes(e *echo.Echo) {
	e.Use(initHydroApiContext)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	addRoute(e, "GET", "/markets", nil, GetMarkets)
	addRoute(e, "GET", "/markets/:marketID/orderbook", &OrderBookReq{}, GetOrderBook)
	addRoute(e, "GET", "/markets/:marketID/trades", &QueryTradeReq{}, GetAllTrades)

	addRoute(e, "GET", "/markets/:marketID/trades/mine", &QueryTradeReq{}, GetAccountTrades, authMiddleware)
	addRoute(e, "GET", "/markets/:marketID/candles", &CandlesReq{}, GetTradingView)
	addRoute(e, "GET", "/fees", &FeesReq{}, GetFees)

	addRoute(e, "GET", "/orders", &QueryOrderReq{}, GetOrders, authMiddleware)
	addRoute(e, "GET", "/orders/:orderID", &QuerySingleOrderReq{}, GetSingleOrder, authMiddleware)
	addRoute(e, "POST", "/orders/build", &BuildOrderReq{}, BuildOrder, authMiddleware)
	addRoute(e, "POST", "/orders", &PlaceOrderReq{}, PlaceOrder, authMiddleware)
	addRoute(e, "DELETE", "/orders/:orderID", &CancelOrderReq{}, CancelOrder, authMiddleware)
	addRoute(e, "GET", "/account/lockedBalances", &LockedBalanceReq{}, GetLockedBalance, authMiddleware)

	// Margin Account Routes
	addRoute(e, "GET", "/margin/accounts/:marketID", &MarginAccountDetailsReq{}, GetMarginAccountDetails, authMiddleware)
	addRoute(e, "POST", "/margin/collateral/deposit", &CollateralManagementReq{}, DepositToCollateral, authMiddleware)
	addRoute(e, "POST", "/margin/collateral/withdraw", &CollateralManagementReq{}, WithdrawFromCollateral, authMiddleware)

	// Loan Management Routes
	addRoute(e, "POST", "/margin/loans/borrow", &CollateralManagementReq{}, BorrowLoan, authMiddleware) // Reusing CollateralManagementReq for borrow
	addRoute(e, "POST", "/margin/loans/repay", &CollateralManagementReq{}, RepayLoan, authMiddleware)    // Reusing CollateralManagementReq for repay
	addRoute(e, "GET", "/margin/loans", &LoanListReq{}, GetLoans, authMiddleware)

	// Margin Position Routes
	addRoute(e, "GET", "/v1/margin/positions", &EmptyReq{}, GetUserMarginPositions, authMiddleware) // New route for listing positions
	addRoute(e, "POST", "/v1/margin/positions/open", &OpenMarginPositionReq{}, OpenMarginPosition, authMiddleware)
	addRoute(e, "POST", "/v1/margin/positions/close", &CloseMarginPositionReq{}, CloseMarginPosition, authMiddleware)
}

func addRoute(e *echo.Echo, method, url string, param Param, handler func(p Param) (interface{}, error), middlewares ...echo.MiddlewareFunc) {
	e.Add(method, url, commonHandler(param, handler), middlewares...)
}

type Response struct {
	Status int         `json:"status"`
	Desc   string      `json:"desc"`
	Data   interface{} `json:"data,omitempty"`
}

func commonResponse(c echo.Context, data interface{}) error {
	return c.String(http.StatusOK, utils.ToJsonString(Response{
		Status: 0,
		Desc:   "success",
		Data:   data,
	}))
}

func errorHandler(err error, c echo.Context) {
	e := c.Echo()

	var status int
	var desc string

	if apiError, ok := err.(*ApiError); ok {
		status = apiError.Code
		desc = apiError.Desc
	} else if errors, ok := err.(validator.ValidationErrors); ok {
		status = -1
		desc = buildErrorMessage(errors)
	} else if e.Debug {
		status = -1
		desc = err.Error()
	} else {
		status = -1
		fmt.Println("err:", err)
		desc = "something wrong"
	}

	// Send response
	if !c.Response().Committed {
		err = c.JSON(http.StatusOK, Response{
			Status: status,
			Desc:   desc,
		})

		if err != nil {
			e.Logger.Error(err)
		}
	}
}

func getEchoServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// open Debugf will return server errors details in json body
	// e.Debugf = true

	e.HTTPErrorHandler = errorHandler

	e.Use(middleware.Logger())
	e.Use(recoverHandler)

	// The BodyDump plugin is used for debug, you can uncomment these lines to see request and response body
	// More details goes https://echo.labstack.com/middleware/body-dump
	//
	// e.Use(middleware.BodyDump(func(c echo.Context, reqBody, resBody []byte) {
	// 	utils.Debugf("Header: %s", c.Request().Header)
	// 	utils.Debugf("Url: %s, Request Body: %s; Response Body: %s", c.Request().RequestURI, string(reqBody), string(resBody))
	// }))

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, "Jwt-Authentication", "Hydro-Authentication"},
	}))

	loadRoutes(e)

	return e
}

var hydro sdk.Hydro

func StartServer(ctx context.Context, startMetric func()) {
	// init redis
	redisClient := connection.NewRedisClient(os.Getenv("HSK_REDIS_URL"))

	// init blockchain
	hydro = ethereum.NewEthereumHydro(os.Getenv("HSK_BLOCKCHAIN_RPC_URL"), os.Getenv("HSK_HYBRID_EXCHANGE_ADDRESS"))

	// Init SDK Wrappers
	// HSK_HYBRID_EXCHANGE_ADDRESS is for the original Hydro exchange contract.
	// HSK_MARGIN_CONTRACT_ADDRESS is for the new Margin contract.
	if err := sdk_wrappers.InitHydroWrappers(os.Getenv("HSK_HYBRID_EXCHANGE_ADDRESS"), os.Getenv("HSK_MARGIN_CONTRACT_ADDRESS")); err != nil {
		panic(fmt.Sprintf("Failed to initialize Hydro SDK Wrappers: %v", err))
	}

	//init database
	models.Connect(os.Getenv("HSK_DATABASE_URL"))

	CacheService, _ = common.InitKVStore(
		&common.RedisKVStoreConfig{
			Ctx:    ctx,
			Client: redisClient,
		},
	)

	QueueService, _ = common.InitQueue(
		&common.RedisQueueConfig{
			Name:   common.HYDRO_ENGINE_EVENTS_QUEUE_KEY,
			Ctx:    ctx,
			Client: redisClient,
		},
	)

	e := getEchoServer()

	s := &http.Server{
		Addr:         ":3001",
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		if err := e.StartServer(s); err != nil {
			e.Logger.Info("shutting down the server: %v", err)
			panic(err)
		}
	}()

	go startMetric()
	<-ctx.Done()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func recoverHandler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}
				stack := make([]byte, 2048)
				length := runtime.Stack(stack, false)
				utils.Errorf("unhandled error: %v %s", err, stack[:length])
				c.Error(err)
			}
		}()
		return next(c)
	}
}
