// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	computev1alpha1 "github.com/ironcore-dev/ironcore/api/compute/v1alpha1"
	irimachinev1alpha1 "github.com/ironcore-dev/ironcore/iri/apis/machine/v1alpha1"
	irimetav1alpha1 "github.com/ironcore-dev/ironcore/iri/apis/meta/v1alpha1"
	metalv1alpha1 "github.com/ironcore-dev/metal/api/v1alpha1"
	metalv1alpha1apply "github.com/ironcore-dev/metal/client/applyconfiguration/api/v1alpha1"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/metal-provider/internal/log"
	"github.com/ironcore-dev/metal-provider/internal/ssa"
	"github.com/ironcore-dev/metal-provider/internal/unix"
)

const (
	MetalProviderFieldManager = "metal.ironcore.dev/provider"
	MetalProviderIRIPrefix    = "iri-"
	MetalProviderSizePrefix   = "metal.ironcore.dev/size-"
)

func NewMetalProviderServer(addr string, namespace string) (*MetalProviderServer, error) {
	return &MetalProviderServer{
		addr:      addr,
		namespace: namespace,
	}, nil
}

type MetalProviderServer struct {
	client.Client
	addr      string
	namespace string
}

// SetupWithManager sets up the server with the Manager.
func (s *MetalProviderServer) SetupWithManager(mgr ctrl.Manager) error {
	s.Client = mgr.GetClient()

	return mgr.Add(s)
}

func (s *MetalProviderServer) Start(ctx context.Context) error {
	ctx = log.WithValues(ctx, "server", "MetalProvider")
	log.Info(ctx, "Starting server")

	ln, err := unix.Listen(ctx, s.addr)
	if err != nil {
		return fmt.Errorf("could not listen to socket %s: %w", s.addr, err)
	}

	srv := grpc.NewServer(grpc.UnaryInterceptor(s.unaryInterceptor(logr.FromContextOrDiscard(ctx))))
	irimachinev1alpha1.RegisterMachineRuntimeServer(srv, s)

	var g *errgroup.Group
	g, ctx = errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Info(ctx, "Listening", "bindAddr", s.addr)
		return srv.Serve(ln)
	})
	g.Go(func() error {
		<-ctx.Done()
		log.Info(ctx, "Stopping server")
		srv.GracefulStop()
		log.Info(ctx, "Server finished")
		return nil
	})
	return g.Wait()
}

func (s *MetalProviderServer) unaryInterceptor(logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(logr.NewContext(ctx, logger), req)
	}
}

func (s *MetalProviderServer) Version(ctx context.Context, _ *irimachinev1alpha1.VersionRequest) (*irimachinev1alpha1.VersionResponse, error) {
	ctx = log.WithValues(ctx, "request", "ListMachines")
	log.Debug(ctx, "Serving")

	return &irimachinev1alpha1.VersionResponse{
		RuntimeName:    "metal-provider",
		RuntimeVersion: "0.0.0",
	}, nil
}

func (s *MetalProviderServer) ListMachines(ctx context.Context, req *irimachinev1alpha1.ListMachinesRequest) (*irimachinev1alpha1.ListMachinesResponse, error) {
	ctx = log.WithValues(ctx, "request", "ListMachines")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "DeleteMachine() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err

	//
	//	filter := req.GetFilter()
	//	id := filter.GetId()
	//	selector := filter.GetLabelSelector()
	//	if id != "" && selector != nil {
	//		err := status.Errorf(codes.InvalidArgument, "machine id and label selectors cannot both be set")
	//		log.Error(ctx, err)
	//		return nil, err
	//	}
	//
	//	var machines []metalv1alpha4.Machine
	//	if id == "" {
	//		pselector, _ := overlayOntoPrefixed(IriLabelPrefix, selector, map[string]string{})
	//		log.Debug(ctx, "Listing machines", "selector", selector)
	//		var machineList metalv1alpha4.MachineList
	//		err := s.List(ctx, &machineList, client.InNamespace(s.namespace), client.MatchingLabels(pselector))
	//		if err != nil {
	//			return nil, internalError(ctx, fmt.Errorf("could not list machines: %w", err))
	//		}
	//
	//		for _, m := range machineList.Items {
	//			if m.Status.Reservation.Status == "Reserved" {
	//				machines = append(machines, m)
	//			}
	//		}
	//	} else {
	//		ctx = log.WithValues(ctx, "machine", id)
	//
	//		log.Debug(ctx, "Getting machine")
	//		var machine metalv1alpha4.Machine
	//		err := s.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: id}, &machine)
	//		if err != nil {
	//			if kerrors.IsNotFound(err) {
	//				err = status.Errorf(codes.NotFound, "machine does not exist")
	//				log.Error(ctx, err)
	//				return nil, err
	//			}
	//			return nil, internalError(ctx, fmt.Errorf("cannot get machine: %w", err))
	//		}
	//
	//		if machine.Status.Reservation.Status != "Reserved" {
	//			err = status.Errorf(codes.NotFound, "machine is not reserved")
	//			log.Error(ctx, err)
	//			return nil, err
	//		}
	//
	//		machines = append(machines, machine)
	//	}
	//
	//	resMachines := make([]*irimachinev1alpha1.Machine, 0, len(machines))
	//	for _, m := range machines {
	//		var bootMap v1.ConfigMap
	//		err := s.Get(ctx, client.ObjectKey{Namespace: m.Namespace, Name: fmt.Sprintf("ipxe-%s", m.Name)}, &bootMap)
	//		if err != nil {
	//			_ = internalError(ctx, fmt.Errorf("reserved machine has no boot configmap: %w", err))
	//			continue
	//		}
	//		image, ok := bootMap.Data["image"]
	//		if !ok {
	//			_ = internalError(ctx, fmt.Errorf("reserved machine has no image in boot configmap"))
	//			continue
	//		}
	//		var ignition []byte
	//		ignition, ok = bootMap.BinaryData["ignition-custom"]
	//		if !ok {
	//			_ = internalError(ctx, fmt.Errorf("reserved machine has no ignition-custom in boot configmap"))
	//			continue
	//		}
	//
	//		resMachines = append(resMachines, &irimachinev1alpha1.Machine{
	//			Metadata: kMetaToMeta(&m.ObjectMeta),
	//			Spec: &irimachinev1alpha1.MachineSpec{
	//				// TODO: Power
	//				Image: &irimachinev1alpha1.ImageSpec{
	//					Image: image,
	//				},
	//				Class:        m.Status.Reservation.Class,
	//				IgnitionData: ignition,
	//				// TODO: Volumes
	//				// TODO: Network
	//			},
	//			Status: &irimachinev1alpha1.MachineStatus{
	//				// TODO: ObservedGeneration
	//				State:    irimachinev1alpha1.MachineState_MACHINE_PENDING,
	//				ImageRef: image,
	//				// TODO: Volumes
	//				// TODO: Network
	//			},
	//		})
	//	}
	//
	//return &irimachinev1alpha1.ListMachinesResponse{
	//	//		Machines: resMachines,
	//}, nil
}

func (s *MetalProviderServer) CreateMachine(ctx context.Context, req *irimachinev1alpha1.CreateMachineRequest) (*irimachinev1alpha1.CreateMachineResponse, error) {
	ctx = log.WithValues(ctx, "request", "CreateMachine")
	log.Debug(ctx, "Serving")

	reqMachine := req.GetMachine()
	reqMetadata := reqMachine.GetMetadata()
	if reqMetadata.GetId() != "" {
		err := status.Errorf(codes.InvalidArgument, "machine id must be empty")
		log.Error(ctx, err)
		return nil, err
	}
	if reqMetadata.GetGeneration() != 0 || reqMetadata.GetCreatedAt() != 0 || reqMetadata.GetDeletedAt() != 0 {
		err := status.Errorf(codes.InvalidArgument, "machine generation, created_at, and deleted_at must all be empty")
		log.Error(ctx, err)
		return nil, err
	}

	reqSpec := reqMachine.GetSpec()
	power, err := iriPowerToPower(reqSpec.Power)
	if err != nil {
		err = status.Errorf(codes.InvalidArgument, "invalid power: %v", reqSpec.Power)
		log.Error(ctx, err)
		return nil, err
	}
	image := reqSpec.GetImage().GetImage()
	class := reqSpec.GetClass()
	if class == "" {
		err = status.Errorf(codes.InvalidArgument, "machine class must be set")
		log.Error(ctx, err)
		return nil, err
	}
	if len(reqSpec.GetVolumes()) != 0 {
		log.Error(ctx, status.Errorf(codes.Unimplemented, "volumes are not supported yet"))
	}
	if len(reqSpec.GetNetworkInterfaces()) != 0 {
		log.Error(ctx, status.Errorf(codes.Unimplemented, "network_interfaces are not supported yet"))
	}
	ctx = log.WithValues(ctx, "power", power, "image", image, "class", class)

	log.Debug(ctx, "Getting machine class")
	var c computev1alpha1.MachineClass
	err = s.Get(ctx, client.ObjectKey{
		Name: class,
	}, &c)
	if err != nil {
		if errors.IsNotFound(err) {
			err = status.Errorf(codes.NotFound, "MachineClass does not exist")
			log.Error(ctx, err)
			return nil, err
		}
		return nil, internalError(ctx, fmt.Errorf("cannot get MachineClass: %w", err))
	}

	var cid uuid.UUID
	cid, err = uuid.NewRandom()
	if err != nil {
		err = status.Errorf(codes.InvalidArgument, "cannot generate UUID: %s", err)
		log.Error(ctx, err)
		return nil, err
	}

	iriAnnotations := addPrefix(MetalProviderIRIPrefix, reqMetadata.Annotations)
	iriLabels := addPrefix(MetalProviderIRIPrefix, reqMetadata.Labels)

	log.Debug(ctx, "Creating ConfigMap")
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cid.String(),
			Namespace: s.namespace,
		},
	}
	cmApply := v1apply.ConfigMap(cm.Name, cm.Namespace). // FIXME - correct?
								WithAnnotations(iriAnnotations).
								WithLabels(iriLabels).
								WithData(map[string]string{
			"image": image,
		}).
		WithBinaryData(map[string][]byte{
			"ignition-custom": reqSpec.GetIgnitionData(),
		})
	err = s.Patch(ctx, &cm, ssa.Apply(cmApply), client.FieldOwner(MetalProviderFieldManager), client.ForceOwnership)
	if err != nil {
		return nil, internalError(ctx, fmt.Errorf("cannot apply ConfigMap: %w", err))
	}

	log.Debug(ctx, "Creating MachineClaim")
	claim := metalv1alpha1.MachineClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cid.String(),
			Namespace: s.namespace,
		},
	}
	apply := metalv1alpha1apply.MachineClaim(claim.Name, claim.Namespace).
		WithAnnotations(iriAnnotations).
		WithLabels(iriLabels).
		WithSpec(metalv1alpha1apply.MachineClaimSpec().
			WithMachineSelector(metav1apply.LabelSelector().
				WithMatchLabels(map[string]string{
					MetalProviderSizePrefix + class: "true",
				})).
			WithImage(image).
			WithPower(power).
			WithIgnitionSecretRef(v1.LocalObjectReference{
				Name: cm.Name,
			}))
	// TODO: Volumes
	// TODO: Network
	err = s.Patch(ctx, &claim, ssa.Apply(apply), client.FieldOwner(MetalProviderFieldManager), client.ForceOwnership)
	if err != nil {
		return nil, internalError(ctx, fmt.Errorf("cannot apply MachineClaim: %w", err))
	}

	log.Info(ctx, "Reserved machine")
	return &irimachinev1alpha1.CreateMachineResponse{
		Machine: &irimachinev1alpha1.Machine{
			Metadata: &irimetav1alpha1.ObjectMetadata{
				Id:          claim.Name,
				Annotations: reqMetadata.Annotations,
				Labels:      reqMetadata.Labels,
			},
			Spec: reqSpec,
			Status: &irimachinev1alpha1.MachineStatus{
				State:    irimachinev1alpha1.MachineState_MACHINE_PENDING,
				ImageRef: image,
				// TODO: Volumes
				// TODO: Network
			},
		},
	}, nil
}

func (s *MetalProviderServer) DeleteMachine(ctx context.Context, req *irimachinev1alpha1.DeleteMachineRequest) (*irimachinev1alpha1.DeleteMachineResponse, error) {
	ctx = log.WithValues(ctx, "request", "DeleteMachine")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "DeleteMachine() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err

	//	id := req.GetMachineId()
	//	if id == "" {
	//		err := status.Errorf(codes.InvalidArgument, "machine id must be specified")
	//		log.Error(ctx, err)
	//		return nil, err
	//	}
	//	ctx = log.WithValues(ctx, "machine", id)
	//
	//	log.Debug(ctx, "Getting machine")
	//	var machine metalv1alpha4.Machine
	//	err := s.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: id}, &machine)
	//	if err != nil {
	//		if kerrors.IsNotFound(err) {
	//			err = status.Errorf(codes.NotFound, "machine does not exist")
	//			log.Error(ctx, err)
	//			return nil, err
	//		}
	//		return nil, internalError(ctx, fmt.Errorf("cannot get machine: %w", err))
	//	}
	//
	//	if machine.Status.Reservation.Status != "Reserved" {
	//		err = status.Errorf(codes.NotFound, "machine is not reserved")
	//		log.Error(ctx, err)
	//		return nil, err
	//	}
	//
	//	// TODO: Power
	//
	//	bootMapApply := v1apply.ConfigMap(fmt.Sprintf("ipxe-%s", machine.Name), machine.Namespace)
	//	bootMap := &v1.ConfigMap{
	//		TypeMeta: metav1.TypeMeta{
	//			APIVersion: *bootMapApply.APIVersion,
	//			Kind:       *bootMapApply.Kind,
	//		},
	//		ObjectMeta: metav1.ObjectMeta{
	//			Namespace: *bootMapApply.Namespace,
	//			Name:      *bootMapApply.Name,
	//		},
	//	}
	//	err = s.Delete(ctx, bootMap)
	//	if err != nil && !kerrors.IsNotFound(err) {
	//		return nil, internalError(ctx, fmt.Errorf("could not delete boot configmap: %w", err))
	//	}
	//
	//	machineApply := metalv1alpha4apply.Machine(machine.Name, machine.Namespace)
	//	annotations, moda := overlayOntoPrefixed(IRILabelPrefix, map[string]string{}, machine.Annotations)
	//	if moda {
	//		machineApply = machineApply.WithAnnotations(annotations)
	//	}
	//	labels, modl := overlayOntoPrefixed(IriLabelPrefix, map[string]string{}, machine.Labels)
	//	if modl {
	//		machineApply = machineApply.WithLabels(labels)
	//	}
	//	if moda || modl {
	//		machine = metalv1alpha4.Machine{
	//			TypeMeta: metav1.TypeMeta{
	//				APIVersion: *machineApply.APIVersion,
	//				Kind:       *machineApply.Kind,
	//			},
	//			ObjectMeta: metav1.ObjectMeta{
	//				Namespace: *machineApply.Namespace,
	//				Name:      *machineApply.Name,
	//			},
	//		}
	//		log.Debug(ctx, "Applying machine annotations and labels")
	//		err = s.Patch(ctx, &machine, patch.ApplyConfiguration(machineApply), client.FieldOwner("metal-provider.ironcore.dev"), client.ForceOwnership)
	//		if err != nil {
	//			return nil, internalError(ctx, fmt.Errorf("could not apply machine: %w", err))
	//		}
	//	}
	//
	//	machineApply = metalv1alpha4apply.Machine(machine.Name, machine.Namespace).WithStatus(metalv1alpha4apply.MachineStatus().WithReservation(metalv1alpha4apply.Reservation().WithStatus("Available")))
	//	machine = metalv1alpha4.Machine{
	//		TypeMeta: metav1.TypeMeta{
	//			APIVersion: *machineApply.APIVersion,
	//			Kind:       *machineApply.Kind,
	//		},
	//		ObjectMeta: metav1.ObjectMeta{
	//			Namespace: *machineApply.Namespace,
	//			Name:      *machineApply.Name,
	//		},
	//	}
	//	log.Debug(ctx, "Applying machine status")
	//	err = s.Client.Status().Patch(ctx, &machine, patch.ApplyConfiguration(machineApply), client.FieldOwner("metal-provider.ironcore.dev"), client.ForceOwnership)
	//	if err != nil {
	//		return nil, internalError(ctx, fmt.Errorf("could not apply machine status: %w", err))
	//	}
	//
	//log.Info(ctx, "Released machine")
	//return &irimachinev1alpha1.DeleteMachineResponse{}, nil
}

func (s *MetalProviderServer) UpdateMachineAnnotations(ctx context.Context, _ *irimachinev1alpha1.UpdateMachineAnnotationsRequest) (*irimachinev1alpha1.UpdateMachineAnnotationsResponse, error) {
	ctx = log.WithValues(ctx, "request", "UpdateMachineAnnotations")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "UpdateMachineAnnotations() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func (s *MetalProviderServer) UpdateMachinePower(ctx context.Context, _ *irimachinev1alpha1.UpdateMachinePowerRequest) (*irimachinev1alpha1.UpdateMachinePowerResponse, error) {
	ctx = log.WithValues(ctx, "request", "UpdateMachinePower")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "UpdateMachinePower() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func (s *MetalProviderServer) AttachVolume(ctx context.Context, _ *irimachinev1alpha1.AttachVolumeRequest) (*irimachinev1alpha1.AttachVolumeResponse, error) {
	ctx = log.WithValues(ctx, "request", "AttachVolume")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "AttachVolume() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func (s *MetalProviderServer) DetachVolume(ctx context.Context, _ *irimachinev1alpha1.DetachVolumeRequest) (*irimachinev1alpha1.DetachVolumeResponse, error) {
	ctx = log.WithValues(ctx, "request", "DetachVolume")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "DetachVolume() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func (s *MetalProviderServer) AttachNetworkInterface(ctx context.Context, _ *irimachinev1alpha1.AttachNetworkInterfaceRequest) (*irimachinev1alpha1.AttachNetworkInterfaceResponse, error) {
	ctx = log.WithValues(ctx, "request", "AttachNetworkInterface")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "AttachNetworkInterface() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func (s *MetalProviderServer) DetachNetworkInterface(ctx context.Context, _ *irimachinev1alpha1.DetachNetworkInterfaceRequest) (*irimachinev1alpha1.DetachNetworkInterfaceResponse, error) {
	ctx = log.WithValues(ctx, "request", "DetachNetworkInterface")
	log.Debug(ctx, "Serving")

	err := status.Errorf(codes.Unimplemented, "DetachNetworkInterface() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func (s *MetalProviderServer) Status(ctx context.Context, _ *irimachinev1alpha1.StatusRequest) (*irimachinev1alpha1.StatusResponse, error) {
	ctx = log.WithValues(ctx, "request", "Status")
	log.Debug(ctx, "Serving")

	log.Debug(ctx, "Listing machines")
	var machines metalv1alpha1.MachineList
	err := s.List(ctx, &machines)
	if err != nil {
		return nil, internalError(ctx, fmt.Errorf("cannot list Machines: %w", err))
	}

	classes := make(map[string]*irimachinev1alpha1.MachineClassStatus)
	for _, m := range machines.Items {
		for l, v := range m.Labels {
			sz, ok := strings.CutPrefix(l, MetalProviderSizePrefix)
			if !ok || v != "true" {
				continue
			}
			ctxx := log.WithValues(ctx, "size", sz)

			var c *irimachinev1alpha1.MachineClassStatus
			c, ok = classes[sz]
			if !ok {
				classes[sz] = nil

				log.Debug(ctxx, "Getting machine class")
				var class computev1alpha1.MachineClass
				err = s.Get(ctx, client.ObjectKey{
					Name: sz,
				}, &class)
				if err != nil {
					if errors.IsNotFound(err) {
						log.Debug(ctxx, "MachineClass does not exist, ignoring")
						continue
					}
					return nil, internalError(ctxx, fmt.Errorf("cannot get MachineClass: %w", err))
				}

				//cpum := class.Capabilities.CPU().MilliValue()
				//var mem int64
				//mem, ok = class.Capabilities.Memory().AsInt64()
				//if !ok {
				//	mem = 0
				//}
				c = &irimachinev1alpha1.MachineClassStatus{
					MachineClass: &irimachinev1alpha1.MachineClass{
						Name: sz,
						// FIXME: check if it is ok to skip this
						//Capabilities: &irimachinev1alpha1.MachineClassCapabilities{
						//	CpuMillis:   cpum,
						//	MemoryBytes: mem,
						//},
					},
				}
				classes[sz] = c
			}
			if c != nil && m.Status.State == "Available" {
				c.Quantity++
			}
		}
	}

	r := &irimachinev1alpha1.StatusResponse{}
	for _, c := range classes {
		if c != nil {
			log.Debug(ctx, "Machine class", "name", c.MachineClass.Name, "quantity", c.Quantity)
			r.MachineClassStatus = append(r.MachineClassStatus, c)
		}
	}
	return r, nil
}

func (s *MetalProviderServer) Exec(ctx context.Context, _ *irimachinev1alpha1.ExecRequest) (*irimachinev1alpha1.ExecResponse, error) {
	err := status.Errorf(codes.Unimplemented, "Exec() has not been implemented yet")
	log.Error(ctx, err)
	return nil, err
}

func internalError(ctx context.Context, err error) error {
	err = status.Errorf(codes.Internal, "%s", err)
	log.Error(ctx, err)
	return err
}

func addPrefix(p string, m map[string]string) map[string]string {
	pm := make(map[string]string, len(m))
	for k, v := range m {
		pm[p+k] = v
	}
	return pm
}

func iriPowerToPower(p irimachinev1alpha1.Power) (metalv1alpha1.Power, error) {
	switch p {
	case irimachinev1alpha1.Power_POWER_OFF:
		return metalv1alpha1.PowerOff, nil
	case irimachinev1alpha1.Power_POWER_ON:
		return metalv1alpha1.PowerOn, nil
	default:
		return "", fmt.Errorf("invalid power value: %v", p)
	}
}

//func overlayOntoPrefixed(prefix string, overlay, prefixed map[string]string) (map[string]string, bool) {
//	mod := false
//
//	for k, v := range overlay {
//		pk := fmt.Sprintf("%s%s", prefix, k)
//		vv, ok := prefixed[pk]
//		if !ok || vv != v {
//			if prefixed == nil {
//				prefixed = make(map[string]string)
//			}
//			prefixed[pk] = v
//			mod = true
//		}
//	}
//
//	lenp := len(prefix)
//	for pk := range prefixed {
//		if !strings.HasPrefix(pk, prefix) {
//			continue
//		}
//		k := pk[lenp:]
//		_, ok := overlay[k]
//		if !ok {
//			delete(prefixed, pk)
//			mod = true
//		}
//	}
//
//	return prefixed, mod
//}

//func extractFromPrefixed(prefix string, prefixed map[string]string) map[string]string {
//	var extracted map[string]string
//	lenp := len(prefix)
//	for pk, v := range prefixed {
//		if !strings.HasPrefix(pk, prefix) {
//			continue
//		}
//		if extracted == nil {
//			extracted = make(map[string]string)
//		}
//		extracted[pk[lenp:]] = v
//	}
//	return extracted
//}

//func kMetaToMeta(meta *metav1.ObjectMeta) *irimetav1alpha1.ObjectMetadata {
//	iriMeta := &irimetav1alpha1.ObjectMetadata{
//		Id:          meta.Name,
//		Annotations: extractFromPrefixed(IriLabelPrefix, meta.Annotations),
//		Labels:      extractFromPrefixed(IriLabelPRefix, meta.Labels),
//		Generation:  meta.Generation,
//		CreatedAt:   meta.CreationTimestamp.Unix(),
//	}
//	if meta.DeletionTimestamp != nil {
//		iriMeta.DeletedAt = meta.DeletionTimestamp.Unix()
//	}
//	return iriMeta
//}
