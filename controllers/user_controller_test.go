package controllers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("UserController", func() {
	var user topology.User
	var userName string

	JustBeforeEach(func() {
		user = topology.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userName,
				Namespace: "default",
			},
			Spec: topology.UserSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	When("creating a user", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				userName = "test-user-http-error"
				fakeRabbitMQClient.PutUserReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("some HTTP error"),
				})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				userName = "test-user-go-error"
				fakeRabbitMQClient.PutUserReturns(nil, errors.New("hit a exception"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("hit a exception"),
				})))
			})
		})

		When("the RabbitMQ Client successfully creates a user", func() {
			BeforeEach(func() {
				userName = "test-user-success"
				fakeRabbitMQClient.PutUserReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("sets the status condition to indicate a success in reconciling", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			It("sets an owner reference and does not block owner deletion", func() {
				user.Name = "test-owner-reference"
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})), "User should have been created and have a True Ready condition")

				generatedSecret := &corev1.Secret{}
				EventuallyWithOffset(1, func() error {
					return client.Get(ctx,
						types.NamespacedName{
							Namespace: user.Namespace,
							Name:      user.Name + "-user-credentials",
						}, generatedSecret)
				}, 10*time.Second).ShouldNot(HaveOccurred())

				for _, ref := range generatedSecret.ObjectMeta.OwnerReferences {
					Expect(ref).To(MatchFields(IgnoreExtras, Fields{
						"BlockOwnerDeletion": PointTo(BeFalse()),
					}))
				}
			})
		})
	})

	When("Deleting a user", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &user)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)

				return user.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				userName = "delete-user-http-error"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete user"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				userName = "delete-user-go-error"
				fakeRabbitMQClient.DeleteUserReturns(nil, errors.New("some error"))
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete user"))
			})
		})

		When("the RabbitMQ Client successfully deletes a user", func() {
			BeforeEach(func() {
				userName = "delete-user-success"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("raises an event to indicate a successful deletion", func() {
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &topology.User{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())

				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete user")),
					ContainElement("Normal SuccessfulDelete successfully deleted user"),
				))
			})
		})
	})
})
