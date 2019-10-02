package tests

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Aggregated role in-action tests", func() {
	var createServiceAccount = func(client kubernetes.Interface, namespace, name string) {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		_, err := client.CoreV1().ServiceAccounts(namespace).Create(sa)
		Expect(err).ToNot(HaveOccurred())
	}

	var createRoleBinding = func(client kubernetes.Interface, clusterRoleName, namespace, serviceAccount string) {
		rb := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceAccount,
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
				APIGroup: "rbac.authorization.k8s.io",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccount,
					Namespace: namespace,
				},
			},
		}

		_, err := client.RbacV1().RoleBindings(namespace).Create(rb)
		Expect(err).ToNot(HaveOccurred())
	}

	f := framework.NewFrameworkOrDie("aggregated-role-tests")

	DescribeTable("admin/edit datavolume permission checks", func(user string) {
		var client cdiClientset.Interface
		var err error

		createServiceAccount(f.K8sClient, f.Namespace.Name, user)
		createRoleBinding(f.K8sClient, user, f.Namespace.Name, user)

		Eventually(func() error {
			client, err = f.GetCdiClientForServiceAccount(f.Namespace.Name, user)
			return err
		}, 60*time.Second, 2*time.Second).ShouldNot(HaveOccurred())

		dv := utils.NewDataVolumeWithHTTPImport("test-"+user, "1Gi", "http://nonexistant.url")
		dv, err = client.Cdi().DataVolumes(f.Namespace.Name).Create(dv)
		Expect(err).ToNot(HaveOccurred())

		dvl, err := client.Cdi().DataVolumes(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dvl.Items).To(HaveLen(1))

		dv, err = client.Cdi().DataVolumes(f.Namespace.Name).Get(dv.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		err = client.Cdi().DataVolumes(f.Namespace.Name).Delete(dv.Name, &metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		dvl, err = client.Cdi().DataVolumes(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dvl.Items).To(HaveLen(0))

		cl, err := client.Cdi().CDIConfigs().List(metav1.ListOptions{})
		fmt.Printf("XXX %+v\n", err)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Items).To(HaveLen(1))

		cfg, err := client.Cdi().CDIConfigs().Get(cl.Items[0].Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		cfg.Spec.ScratchSpaceStorageClass = &[]string{"foobar"}[0]
		cfg, err = client.Cdi().CDIConfigs().Update(cfg)
		Expect(err).To(HaveOccurred())
	},
		Entry("can do everything with admin", "admin"),
		Entry("can do everything with edit", "edit"),
	)

	It("view datavolume permission checks", func() {
		const user = "view"
		var client cdiClientset.Interface
		var err error

		createServiceAccount(f.K8sClient, f.Namespace.Name, user)
		createRoleBinding(f.K8sClient, user, f.Namespace.Name, user)

		Eventually(func() error {
			client, err = f.GetCdiClientForServiceAccount(f.Namespace.Name, user)
			return err
		}, 60*time.Second, 2*time.Second).ShouldNot(HaveOccurred())

		dv := utils.NewDataVolumeWithHTTPImport("test-"+user, "1Gi", "http://nonexistant.url")
		dv, err = client.Cdi().DataVolumes(f.Namespace.Name).Create(dv)
		Expect(err).To(HaveOccurred())

		dvl, err := client.Cdi().DataVolumes(f.Namespace.Name).List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(dvl.Items).To(HaveLen(0))

		_, err = client.Cdi().DataVolumes(f.Namespace.Name).Get("test-"+user, metav1.GetOptions{})
		Expect(err).To(HaveOccurred())
		Expect(k8serrors.IsNotFound(err)).To(BeTrue())

		cl, err := client.Cdi().CDIConfigs().List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Items).To(HaveLen(1))

		cfg, err := client.Cdi().CDIConfigs().Get(cl.Items[0].Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		cfg.Spec.ScratchSpaceStorageClass = &[]string{"foobar"}[0]
		cfg, err = client.Cdi().CDIConfigs().Update(cfg)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Aggregated role definition tests", func() {
	var adminRules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes/source",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"patch",
				"update",
			},
		},
		{
			APIGroups: []string{
				"upload.cdi.kubevirt.io",
			},
			Resources: []string{
				"uploadtokenrequests",
			},
			Verbs: []string{
				"*",
			},
		},
	}

	var editRules = adminRules

	var viewRules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"datavolumes/source",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"cdi.kubevirt.io",
			},
			Resources: []string{
				"cdiconfigs",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	f := framework.NewFrameworkOrDie("aggregated-role-definition-tests")

	DescribeTable("check all expected rules exist", func(role string, rules []rbacv1.PolicyRule) {
		clusterRole, err := f.K8sClient.RbacV1().ClusterRoles().Get(role, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		found := false
		for _, expectedRule := range rules {
			for _, r := range clusterRole.Rules {
				if reflect.DeepEqual(expectedRule, r) {
					found = true
					break
				}
			}
		}
		Expect(found).To(BeTrue())
	},
		Entry("for admin", "admin", adminRules),
		Entry("for edit", "edit", editRules),
		Entry("for view", "view", viewRules),
	)
})