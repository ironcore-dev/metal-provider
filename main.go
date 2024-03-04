// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	computev1alpha1 "github.com/ironcore-dev/ironcore/api/compute/v1alpha1"
	"github.com/ironcore-dev/metal-provider/internal/log"
	"github.com/ironcore-dev/metal-provider/servers"
	metalv1alpha4 "github.com/ironcore-dev/metal/apis/metal/v1alpha4"
)

func usage() {
	name := filepath.Base(os.Args[0])
	_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [--option]...\n", name)
	_, _ = fmt.Fprintf(os.Stderr, "Options:\n")
	pflag.PrintDefaults()
}

func exitUsage(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s: %s\n", filepath.Base(os.Args[0]), err)
	}
	pflag.Usage()
	os.Exit(2)
}

type params struct {
	dev                    bool
	leaderElect            bool
	healthProbeBindAddress string
	metricsBindAddress     string
	kubeconfig             string
	namespace              string
	gRPCAddr               string
}

func parseCmdLine() params {
	pflag.Usage = usage
	pflag.ErrHelp = nil
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)

	pflag.Bool("dev", false, "Log human-readable messages at debug level.")
	pflag.Bool("leader-elect", false, "Enable leader election for controller manager to ensure there is only one active controller manager.")
	pflag.String("health-probe-bind-address", "", "The address that the health probe server will listen on.")
	pflag.String("metrics-bind-address", "0", "The address that the metrics server will listen on.")
	pflag.String("kubeconfig", "", "Use a kubeconfig to run out of cluster.")
	pflag.String("namespace", "", "Limit monitoring to a specific namespace.")
	pflag.String("grpc-address", "/run/metal-provider.sock", "The address that the gRPC server will listen on.")

	var help bool
	pflag.BoolVarP(&help, "help", "h", false, "Show this help message.")
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		exitUsage(err)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	err = pflag.CommandLine.Parse(os.Args[1:])
	if err != nil {
		exitUsage(err)
	}
	if help {
		exitUsage(nil)
	}

	return params{
		dev:                    viper.GetBool("dev"),
		leaderElect:            viper.GetBool("leader-elect"),
		healthProbeBindAddress: viper.GetString("health-probe-bind-address"),
		metricsBindAddress:     viper.GetString("metrics-bind-address"),
		kubeconfig:             viper.GetString("kubeconfig"),
		namespace:              viper.GetString("namespace"),
		gRPCAddr:               viper.GetString("grpc-address"),
	}
}

func main() {
	p := parseCmdLine()

	var exitCode int
	defer func() { os.Exit(exitCode) }()

	ctx, stop := signal.NotifyContext(log.Setup(context.Background(), p.dev, false, os.Stderr), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	defer stop()
	log.Info(ctx, "Starting ORI machine provider")

	defer func() { log.Info(ctx, "Exiting", "exitCode", exitCode) }()

	l := logr.FromContextOrDiscard(ctx)
	klog.SetLogger(l)
	ctrl.SetLogger(l)

	if p.namespace == "" {
		ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil && !os.IsNotExist(err) {
			log.Error(ctx, fmt.Errorf("cannot determine in-cluster namespace: %w", err))
			exitCode = 1
			return
		}
		p.namespace = string(ns)
		if p.namespace == "" {
			log.Error(ctx, fmt.Errorf("namespace must be specified when running outside of a Kubernetes cluster"))
			exitCode = 1
			return
		}
		log.Debug(ctx, "Using in-cluster namespace", "namespace", p.namespace)
	}

	log.Debug(ctx, "Loading kubeconfig")
	kcfg, err := ctrl.GetConfig()
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot load kubeconfig: %w", err))
		exitCode = 1
		return
	}

	scheme := runtime.NewScheme()
	err = kscheme.AddToScheme(scheme)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create type scheme: %w", err))
		exitCode = 1
		return
	}
	err = computev1alpha1.AddToScheme(scheme)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create type scheme: %w", err))
		exitCode = 1
		return
	}
	err = metalv1alpha4.AddToScheme(scheme)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create type scheme: %w", err))
		exitCode = 1
		return
	}
	err = metalv1alpha4.AddToScheme(scheme)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create type scheme: %w", err))
		exitCode = 1
		return
	}

	var mgr manager.Manager
	mgr, err = ctrl.NewManager(kcfg, ctrl.Options{
		BaseContext: func() context.Context {
			return ctx
		},
		Scheme:                 scheme,
		LeaderElection:         p.leaderElect,
		LeaderElectionID:       "metal-provider.ironcore.dev",
		HealthProbeBindAddress: p.healthProbeBindAddress,
		Metrics: server.Options{
			BindAddress: p.metricsBindAddress,
		},
	})
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create manager: %w", err))
		exitCode = 1
		return
	}

	err = mgr.AddHealthzCheck("health", healthz.Ping)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot set up health check: %w", err))
		exitCode = 1
		return
	}

	err = mgr.AddReadyzCheck("check", healthz.Ping)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot set up ready check: %w", err))
		exitCode = 1
		return
	}

	var grpcServer *servers.GRPCServer
	grpcServer, err = servers.NewGRPCServer(p.gRPCAddr, p.namespace)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create server: %w", err), "server", "gRPC")
		exitCode = 1
		return
	}
	err = grpcServer.SetupWithManager(mgr)
	if err != nil {
		log.Error(ctx, fmt.Errorf("cannot create server: %w", err), "server", "gRPC")
		exitCode = 1
		return
	}

	log.Info(ctx, "Starting manager")
	err = mgr.Start(ctx)
	if err != nil {
		log.Error(ctx, err)
		exitCode = 1
		return
	}
}
