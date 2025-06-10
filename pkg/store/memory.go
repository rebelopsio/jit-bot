package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/rebelopsio/jit-bot/pkg/models"
)

type MemoryStore struct {
	mu       sync.RWMutex
	clusters map[string]*models.Cluster
	accesses map[string]*models.ClusterAccess
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		clusters: make(map[string]*models.Cluster),
		accesses: make(map[string]*models.ClusterAccess),
	}
}

func (s *MemoryStore) CreateCluster(cluster *models.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clusters[cluster.ID]; exists {
		return fmt.Errorf("cluster %s already exists", cluster.ID)
	}

	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()
	s.clusters[cluster.ID] = cluster
	return nil
}

func (s *MemoryStore) GetCluster(id string) (*models.Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, exists := s.clusters[id]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", id)
	}
	return cluster, nil
}

func (s *MemoryStore) ListClusters() ([]*models.Cluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clusters := make([]*models.Cluster, 0, len(s.clusters))
	for _, cluster := range s.clusters {
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func (s *MemoryStore) UpdateCluster(cluster *models.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clusters[cluster.ID]; !exists {
		return fmt.Errorf("cluster %s not found", cluster.ID)
	}

	cluster.UpdatedAt = time.Now()
	s.clusters[cluster.ID] = cluster
	return nil
}

func (s *MemoryStore) DeleteCluster(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clusters[id]; !exists {
		return fmt.Errorf("cluster %s not found", id)
	}

	delete(s.clusters, id)
	return nil
}

func (s *MemoryStore) CreateAccess(access *models.ClusterAccess) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.accesses[access.ID]; exists {
		return fmt.Errorf("access %s already exists", access.ID)
	}

	s.accesses[access.ID] = access
	return nil
}

func (s *MemoryStore) GetAccess(id string) (*models.ClusterAccess, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	access, exists := s.accesses[id]
	if !exists {
		return nil, fmt.Errorf("access %s not found", id)
	}
	return access, nil
}

func (s *MemoryStore) ListUserAccesses(userID string) ([]*models.ClusterAccess, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accesses := make([]*models.ClusterAccess, 0)
	for _, access := range s.accesses {
		if access.UserID == userID {
			accesses = append(accesses, access)
		}
	}
	return accesses, nil
}
