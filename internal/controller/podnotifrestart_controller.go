package controllers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/NoNickeD/pod-restart-notifier-operator-crd/api/v1"

	corev1 "k8s.io/api/core/v1"
)

// Notifier interface
type Notifier interface {
	Notify(message string) error
}

// DiscordNotifier struct
type DiscordNotifier struct {
	WebhookURL string
}

func (d *DiscordNotifier) Notify(message string) error {
	payload := fmt.Sprintf(`{"content": "%s"}`, message)
	return postMessage(d.WebhookURL, payload)
}

// TeamsNotifier struct
type TeamsNotifier struct {
	WebhookURL string
}

func (t *TeamsNotifier) Notify(message string) error {
	payload := fmt.Sprintf(`{
		"@type": "MessageCard",
		"@context": "http://schema.org/extensions",
		"summary": "Pod Restart Notification",
		"themeColor": "0078D7",
		"text": "%s"
	}`, message)
	return postMessage(t.WebhookURL, payload)
}

// SlackNotifier struct
type SlackNotifier struct {
	WebhookURL string
}

func (s *SlackNotifier) Notify(message string) error {
	payload := fmt.Sprintf(`{"text": "%s"}`, message)
	return postMessage(s.WebhookURL, payload)
}

// PodNotifRestartReconciler struct
type PodNotifRestartReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile function
func (r *PodNotifRestartReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("podnotifrestart", req.NamespacedName)

	var pnr monitoringv1.PodNotifRestart
	if err := r.Client.Get(ctx, req.NamespacedName, &pnr); err != nil {
		if errors.IsNotFound(err) {
			// If the resource is not found, it might have been deleted
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PodNotifRestart")
		return ctrl.Result{}, err
	}

	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList, client.InNamespace(pnr.Namespace)); err != nil {
		log.Error(err, "unable to list pods")
		return ctrl.Result{}, err
	}

	// Initialize notifiers
	discord := &DiscordNotifier{WebhookURL: os.Getenv("DISCORD_WEBHOOK_URL")}
	teams := &TeamsNotifier{WebhookURL: os.Getenv("TEAMS_WEBHOOK_URL")}
	slack := &SlackNotifier{WebhookURL: os.Getenv("SLACK_WEBHOOK_URL")}

	for _, pod := range podList.Items {
		for _, status := range pod.Status.ContainerStatuses {
			if status.RestartCount >= pnr.Spec.MinRestarts {
				message := fmt.Sprintf("Pod %s has restarted %d times", pod.Name, status.RestartCount)

				// Adding log line to output restart information
				log.Info("Sending restart notification", "pod", pod.Name, "restartCount", status.RestartCount)

				if err := sendNotification(message, discord, teams, slack); err != nil {
					log.Error(err, "failed to send notification")
					return ctrl.Result{}, err
				}
			}
		}
	}

	// Requeue the request to check again in 2 minutes
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

// SetupWithManager function
func (r *PodNotifRestartReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PodNotifRestart{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// sendNotification function
func sendNotification(message string, notifiers ...Notifier) error {
	var lastError error
	for _, notifier := range notifiers {
		if emptyNotifier(notifier) {
			continue
		}
		if err := notifier.Notify(message); err != nil {
			lastError = err
			fmt.Println("Error sending notification:", err)
		}
	}
	return lastError
}

func emptyNotifier(notifier Notifier) bool {
	switch n := notifier.(type) {
	case *DiscordNotifier:
		return n.WebhookURL == ""
	case *TeamsNotifier:
		return n.WebhookURL == ""
	case *SlackNotifier:
		return n.WebhookURL == ""
	default:
		return true
	}
}

// postMessage function
func postMessage(webhookURL string, payload string) error {
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBufferString(payload))
	if err != nil {
		return fmt.Errorf("failed to send post request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK HTTP status: %s", resp.Status)
	}

	return nil
}
