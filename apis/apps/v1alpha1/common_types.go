/*
Copyright 2024 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

// ObjectStorageType represents the object storage type.
type ObjectStorageType string

const (
	// ObjectStorageS3 represents all storage that compatible with the Amazon S3.
	ObjectStorageS3 ObjectStorageType = "s3"
	// ObjectStorageGS represents the Google storage
	ObjectStorageGS ObjectStorageType = "gs"
	// ObjectStorageUnknown represents the unknown storage type
	ObjectStorageUnknown ObjectStorageType = "unknown"
)

type StorageProvider struct {
	S3 *S3StorageProvider `json:"s3,omitempty"`
	GS *GsStorageProvider `json:"gs,omitempty"`
}

// S3StorageProvider represents a S3 compliant storage for storing backups.
type S3StorageProvider struct {
	// Region in which the S3 compatible bucket is located.
	Region string `json:"region,omitempty"`
	// Bucket in which to store the backup data.
	Bucket string `json:"bucket,omitempty"`
	// Endpoint of S3 compatible storage service
	Endpoint string `json:"endpoint,omitempty"`
	// SecretName is the name of secret which stores access key and secret key.
	// Secret keys: access-key, secret-key
	SecretName string `json:"secretName,omitempty"`
}

// GsStorageProvider represents a GS compliant storage for storing backups.
type GsStorageProvider struct {
	// Location in which the gs bucket is located.
	Location string `json:"location,omitempty"`
	// Bucket in which to store the backup data.
	Bucket string `json:"bucket,omitempty"`
	// SecretName is the name of secret which stores
	// the GS service account or refresh token JSON.
	// Secret key: credentials
	SecretName string `json:"secretName,omitempty"`
}

// NamespacedObjectReference contains enough information to let you locate
// the referenced object in any namespace
type NamespacedObjectReference struct {
	// ClusterName of backup/restore cluster
	ClusterName string `json:"clusterName"`

	// ClusterNamespace of backup/restore cluster
	// +optional
	ClusterNamespace *string `json:"clusterNamespace,omitempty"`
}
