package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/term"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

// GetSafeLockName takes a provided lock name and attempts to convert it into one
// that is safe to use with a Serializer instance. If it is unable to convert the
// provided name, it returns an error.
func GetSafeLockName(lockName string) (string, error) {
	if len(lockName) == 0 {
		return "", trace.Errorf("lock name is empty")
	}

	if len(lockName) > 63 {
		return "", trace.Errorf("lock name %q is too long (max 63 characters)", lockName)
	}

	lockRunes := []rune(lockName)
	if !unicode.IsLetter(lockRunes[0]) {
		return "", trace.Errorf("lock name %q must start with a letter", lockName)
	}

	lastRune := lockRunes[len(lockRunes)-1]
	if !unicode.IsLetter(lastRune) && !unicode.IsDigit(lastRune) {
		return "", trace.Errorf("lock name %q must end with a letter or digit", lockName)
	}

	for i, lockRune := range lockRunes {
		// Make all letters lowercase
		if unicode.IsUpper(lockRune) {
			lockRunes[i] = unicode.ToLower(lockRune)
			continue
		}

		// Replace symbols with `-`
		if !unicode.IsLetter(lockRune) && unicode.IsDigit(lockRune) && lockRune != '-' {
			lockRunes[i] = '-'
		}
	}

	return string(lockRunes), nil
}

type Serializer interface {
	// TakeSerializationLock attempts to take the serialization lock `lockName`.
	// This is a blocking function. It supports cancellation via the context.
	// The lock name must be a single DNS label as defined by RFC 1035.
	// The returned function _must_ be called once all tasks requiring serialization
	// are complete. If it is not called, then the lock will remain held by this process
	// for a long period of time (weeks or more), blocking other processes.
	TakeSerializationLock(ctx context.Context, lockName string) (func(), error)
}

type kubernetesSerializerLeaderElectionConfig struct {
	enable        bool
	namespace     string
	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
}

type kubernetesSerializerConfig struct {
	explicitConfigPath   string
	k8sConfigOverrides   clientcmd.ConfigOverrides
	leaderElectionConfig kubernetesSerializerLeaderElectionConfig
}

type kubernetesSerializer struct {
	kubernetesSerializerConfig
	hasAttemptedToLock atomic.Bool
}

// Based on https://kubernetes.io/docs/reference/command-line-tools-reference/kube-controller-manager/
func (kslec *kubernetesSerializerLeaderElectionConfig) registerFlags(fs *flag.FlagSet) {
	fs.BoolVar(&kslec.enable, "leader-elect", true, "WARNING: this should only be set to 'false' in local development environments. "+
		"True if only once instance of the tool should run per package type, false otherwise.")
	fs.StringVar(&kslec.namespace, "leader-election-namespace", "", "Namespace used to perform leader election. "+
		"This should be set if leader election is enabled.")
	// Data safety is absolutely paramount. Set this to a very long time so that even if a process crashes, loses permissions, or can't
	// talk with the k8s API, other runs are blocked. If this happens, manual investigation is both desired and required to resolve the
	// problem.
	fs.DurationVar(&kslec.leaseDuration, "leader-election-lease-duration", 30*24*time.Hour, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")

	// The maximum time between when a process gives up election and another process takes it is the renewal time + the retry period.
	fs.DurationVar(&kslec.renewDeadline, "leader-election-renew-deadline", time.Minute, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	fs.DurationVar(&kslec.retryPeriod, "leader-election-retry-period", 5*time.Second, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
}

func (ksc *kubernetesSerializerConfig) registerFlags(fs *flag.FlagSet) {
	fs.StringVar(&ksc.explicitConfigPath, clientcmd.RecommendedConfigPathFlag, "", "Path to the kubeconfig file to use for CLI requests.")

	ksc.leaderElectionConfig.registerFlags(fs)
}

func NewKubernetesSerializer(fs *flag.FlagSet) *kubernetesSerializer {
	ks := &kubernetesSerializer{}
	ks.registerFlags(fs)

	return ks
}

// Get the cluster configurations from the following sources, in order of precedence:
// 1. A provided "kubeconfig" flag
// 2. The default kubeconfig file, at ~/.kube/config
// 3. The in-cluster kubeconfig, if running in a pod
func (ks *kubernetesSerializer) getClusterClient() (*kubernetes.Clientset, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = ks.explicitConfigPath

	var clientConfig clientcmd.ClientConfig
	if term.IsTerminal(int(os.Stdin.Fd())) {
		clientConfig = clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &ks.k8sConfigOverrides, os.Stdin)
	} else {
		clientConfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &ks.k8sConfigOverrides)
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, trace.Wrap(err, "failed to build k8s client config")
	}

	return kubernetes.NewForConfig(config)
}

// Generates a unique ID for the host.
func (ks *kubernetesSerializer) generateID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", trace.Wrap(err, "error getting hostname")
	}

	return fmt.Sprintf("%s-%s", hostname, uuid.New()), nil
}

func (ks *kubernetesSerializer) isFirstRun() bool {
	return ks.hasAttemptedToLock.CompareAndSwap(false, true)
}

// TakeSerializationLock is used to ensure that only once instance of the tool is ran at a time.
// The release channel causes the lease to be relinquished when closed. It should be closed after all tasks that require serialization
// are complete. Failure to do so will block other instances of this tool, but should "fail safe" and avoid data corruption.
// Lock name should be the runner name, i.e "apt" or "yum".
func (ks *kubernetesSerializer) TakeSerializationLock(ctx context.Context, lockName string) (func(), error) {
	if !ks.leaderElectionConfig.enable {
		warnMsg := "WARNING: LEADER ELECTION HAS BEEN DISABLED.\n" +
			"This is highly likely to cause data corruption outside of local development environments. \n" +
			"Waiting 60s before continuing to provide the opportunity to cancel.\n"
		slog.WarnContext(ctx, warnMsg)

		timer := time.NewTimer(time.Minute)
		select {
		case <-ctx.Done():
			return nil, trace.Errorf("cancelling non-serialized lock")
		case <-timer.C:
		}

		return func() {}, nil
	}

	if ks.leaderElectionConfig.namespace == "" {
		return nil, trace.Errorf("leadership namespace was not configured")
	}

	// Only allow this to be called once per tool run
	// Allowing this to be called multiple times without erroring is complex. As
	// there isn't really any need for this currently, this simple approach
	// handles the current need without additional complexity.
	if !ks.isFirstRun() {
		return nil, trace.Errorf("already attempted to take the leadership lock")
	}

	// Setup the leadership lock
	lock, err := ks.buildLock(lockName)
	if err != nil {
		return nil, trace.Wrap(err, "failed to build leadership election lock")
	}

	// Wait for the leadership lock
	releaseLockCallback, err := ks.waitForLock(ctx, lock)
	if err != nil {
		return nil, trace.Wrap(err, "failed to acquire leadership election lock")
	}

	return releaseLockCallback, nil
}

func (ks *kubernetesSerializer) buildLock(lockName string) (*resourcelock.LeaseLock, error) {
	k8sClient, err := ks.getClusterClient()
	if err != nil {
		return nil, trace.Wrap(err, "failed to create k8s client")
	}

	id, err := ks.generateID()
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate election ID")
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(slog.Debug)
	eventBroadcaster.StartRecordingToSink(&clientv1.EventSinkImpl{Interface: k8sClient.CoreV1().Events(ks.leaderElectionConfig.namespace)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: lockName})

	return &resourcelock.LeaseLock{
		LeaseMeta: v1.ObjectMeta{
			Name:      lockName,
			Namespace: ks.leaderElectionConfig.namespace,
		},
		Client: k8sClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		},
	}, nil
}

func (ks *kubernetesSerializer) buildLeaderElector(lock *resourcelock.LeaseLock) (*leaderelection.LeaderElector, chan any, error) {
	// This channel will be closed when this instance successfully takes the serialization lock
	elected := make(chan any)

	startedLeading := false
	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Name:            lock.LeaseMeta.Name,
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   ks.leaderElectionConfig.leaseDuration,
		RenewDeadline:   ks.leaderElectionConfig.renewDeadline,
		RetryPeriod:     ks.leaderElectionConfig.retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				slog.InfoContext(ctx, "successfully received serialization lock")
				startedLeading = true
				close(elected)
			},
			OnNewLeader: func(identity string) {
				slog.InfoContext(context.Background(), "a leader was elected", "identity", identity)
			},
			OnStoppedLeading: func() {
				// This function is called regardless of whether or not the lock was ever actually required.
				// It can be called if the context was cancelled while waiting for the lock.
				if startedLeading {
					slog.InfoContext(context.Background(), "relinquishing lock")
				}
			},
		},
	}

	leaderElector, err := leaderelection.NewLeaderElector(leaderElectionConfig)
	if err != nil {
		close(elected)
		return nil, nil, trace.Wrap(err, "failed to create new leader elector")
	}

	return leaderElector, elected, nil
}

func (ks *kubernetesSerializer) waitForLock(ctx context.Context, lock *resourcelock.LeaseLock) (context.CancelFunc, error) {
	leaderElector, elected, err := ks.buildLeaderElector(lock)
	if err != nil {
		return nil, err
	}

	// The elector needs a special context that ignores the main context's cancellation.
	// The leader lock should be held until the caller indicates that everything that requires
	// serialization is complete, including any cleanup tasks that occur when the main context
	// is cancelled. It is the caller's responsibility to close the release channel once the
	// main context is cancelled and all cleanup tasks are complete.
	electionCtx, releaseLockCallback := context.WithCancel(context.Background())

	slog.InfoContext(ctx, "starting leader election", "identity", lock.LockConfig.Identity)

	// Start leader election, waiting for the lock (leader lease) before continuing
	go leaderElector.Run(electionCtx)

	select {
	case <-ctx.Done():
		// Ensure that the lease is relinquished in the event that it was received between when the context
		// was cancelled, and this was hit
		releaseLockCallback()
		return nil, ctx.Err()
	case <-elected:
	}

	return releaseLockCallback, nil
}
