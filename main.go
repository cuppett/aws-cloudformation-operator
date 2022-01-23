/*
MIT License

Copyright (c) 2018 Martin Linkhorst
Copyright (c) 2022 Stephen Cuppett

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"flag"
	cfv1alpha1 "github.com/cuppett/aws-cloudformation-controller/apis/cloudformation.services.k8s.aws/v1alpha1"
	configv1alpha1 "github.com/cuppett/aws-cloudformation-controller/apis/services.k8s.aws/v1alpha1"
	"github.com/cuppett/aws-cloudformation-controller/controllers/cloudformation.services.k8s.aws"
	servicesk8saws "github.com/cuppett/aws-cloudformation-controller/controllers/services.k8s.aws"
	configv1 "github.com/openshift/api/config/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme       = runtime.NewScheme()
	setupLog     = ctrl.Log.WithName("setup")
	StackFlagSet *pflag.FlagSet
)

func init() {

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensions.AddToScheme(scheme))
	utilruntime.Must(configv1.Install(scheme))
	utilruntime.Must(cfv1alpha1.AddToScheme(scheme))
	utilruntime.Must(configv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	StackFlagSet = pflag.NewFlagSet("stack", pflag.ExitOnError)
	StackFlagSet.String("assume-role", "", "Assume AWS role when defined. Useful for stacks in another AWS account. Specify the full ARN, e.g. `arn:aws:iam::123456789:role/cloudformation-controller`")
	StackFlagSet.StringToString("tag", map[string]string{}, "Tags to apply to all Stacks by default. Specify multiple times for multiple tags.")
	StackFlagSet.Bool("dry-run", false, "If true, don't actually do anything.")
	StackFlagSet.Bool("no-webhook", false, "If true, don't run the webhook server.")
}

func main() {
	var namespace string
	var watchNamespaces []string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var err error

	flag.StringVar(&namespace, "namespace", "", "The Kubernetes namespace to watch")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.CommandLine.AddFlagSet(StackFlagSet)
	pflag.Parse()

	if namespace == "" {
		namespace = os.Getenv("WATCH_NAMESPACE")
	}

	if namespace != "" {
		watchNamespaces = strings.Split(namespace, ",")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "3680e595.cuppett.dev",
		Namespace:              namespace, // namespaced-scope when the value is not an empty string
	}
	// Add support for MultiNamespace set in WATCH_NAMESPACE (e.g ns1,ns2)
	if len(watchNamespaces) > 0 {
		setupLog.Info("manager set up with multiple namespaces", "namespaces", namespace)
		cacheNamespaces := watchNamespaces
		configNamespace, exists := os.LookupEnv("POD_NAMESPACE")
		if exists {
			cacheNamespaces = append(cacheNamespaces, configNamespace)
		}
		// configure cluster-scoped with MultiNamespacedCacheBuilder
		options.Namespace = ""
		options.NewCache = cache.MultiNamespacedCacheBuilder(cacheNamespaces)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	assumeRole, err := StackFlagSet.GetString("assume-role")
	if err != nil {
		setupLog.Error(err, "error parsing flag")
		os.Exit(1)
	}
	defaultTags, err := StackFlagSet.GetStringToString("tag")
	if err != nil {
		setupLog.Error(err, "error parsing flag")
		os.Exit(1)
	}

	dryRun, err := StackFlagSet.GetBool("dry-run")
	if err != nil {
		setupLog.Error(err, "error parsing flag")
		os.Exit(1)
	}

	configReconciler := servicesk8saws.InitializeConfigReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("workers").WithName("Config"),
		mgr.GetScheme(),
		assumeRole,
	)
	if err = configReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Config")
		os.Exit(1)
	}

	cfHelper := &cloudformation_services_k8s_aws.CloudFormationHelper{
		ConfigReconciler: configReconciler,
	}

	channelHub := &cloudformation_services_k8s_aws.ChannelHub{
		MappingChannel: make(chan *cfv1alpha1.Stack),
		FollowChannel:  make(chan *cfv1alpha1.Stack),
	}

	mapWriter := &cloudformation_services_k8s_aws.MapWriter{
		Client:     mgr.GetClient(),
		Log:        ctrl.Log.WithName("workers").WithName("Stack"),
		ChannelHub: *channelHub,
		Scheme:     mgr.GetScheme(),
	}
	go mapWriter.Worker()

	stackFollower := &cloudformation_services_k8s_aws.StackFollower{
		Client:               mgr.GetClient(),
		Log:                  ctrl.Log.WithName("workers").WithName("Stack"),
		ChannelHub:           *channelHub,
		CloudFormationHelper: cfHelper,
		StacksFollowing: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cloudformation_stacks_following",
				Help: "Number of CloudFormation stacks being followed currently",
			},
		),
		StacksFollowed: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cloudformation_stacks_followed",
				Help: "Total number of CloudFormation stacks followed (lifetime)",
			},
		),
	}
	go stackFollower.Receiver()
	go stackFollower.Worker()
	metrics.Registry.MustRegister(stackFollower.StacksFollowing)
	metrics.Registry.MustRegister(stackFollower.StacksFollowed)

	if err = (&cloudformation_services_k8s_aws.StackReconciler{
		Client:               mgr.GetClient(),
		ChannelHub:           *channelHub,
		Log:                  ctrl.Log.WithName("controllers").WithName("Stack"),
		Scheme:               mgr.GetScheme(),
		WatchNamespaces:      watchNamespaces,
		CloudFormationHelper: cfHelper,
		DefaultTags:          defaultTags,
		DryRun:               dryRun,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Stack")
		os.Exit(1)
	}

	noWebHook, err := StackFlagSet.GetBool("no-webhook")
	if err != nil {
		setupLog.Error(err, "error parsing flag")
		os.Exit(1)
	}
	if !noWebHook {
		if err = (&cfv1alpha1.Stack{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Stack")
			os.Exit(1)
		}
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
