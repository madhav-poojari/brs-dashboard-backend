# Referral Network Graph - API Design Plan

## Overview

This plan outlines the API architecture for building a network visualization system that displays referrer-referee relationships with geographic clustering and relationship metadata.

---

## 1. DATABASE SCHEMA REQUIREMENTS

### New Tables Needed

#### A. `referral_relationships` Table

Stores the connections between referrers and referees.

**Columns:**

- `id` (UUID, Primary Key) - Unique relationship identifier
- `referrer_id` (String, FK to User.id) - Person who referred another
- `referee_id` (String, FK to User.id) - Person who was referred
- `relationship_type` (Enum/String) - How they know each other:
  - `vendor`
  - `classmate_college`
  - `classmate_school`
  - `coworker`
  - `family`
  - `friend`
  - `coach`
  - `student`
  - `other`
- `relationship_description` (Text, nullable) - Custom description if "other"
- `created_at` (Timestamp) - When relationship was recorded
- `updated_at` (Timestamp)
- `created_by` (String, FK to User.id) - Admin who added this relationship

**Indexes:**

- `(referrer_id, referee_id)` - Composite unique index
- `referrer_id` - For finding all referees of a person
- `referee_id` - For finding who referred a person
- `relationship_type` - For filtering by relationship

---

## 2. DATA MODELS (Response Objects)

### Node Response Model

```
ReferralNode {
  id: string (user_id)
  name: string (first_name + last_name)
  state: string
  city: string
  role: string (coach/mentor/student/etc)
  profile_picture_url: string
  coordinates?: {
    x: number (for front-end force-directed layout - NOT stored, calculated)
    y: number
  }
}
```

### Edge/Relationship Response Model

```
ReferralEdge {
  id: string (relationship_id)
  source_id: string (referrer_id)
  target_id: string (referee_id)
  relationship_type: string
  relationship_description?: string
  created_at: timestamp
  created_by_admin: string
}
```

### Full Graph Response Model

```
ReferralGraph {
  nodes: ReferralNode[]
  edges: ReferralEdge[]
  metadata: {
    total_nodes: int
    total_edges: int
    states_count: Map<state, int>
    relationship_types_summary: Map<type, int>
  }
}
```

### Node Detail Response Model (Hover Info)

```
NodeDetail {
  user_id: string
  full_name: string
  email: string
  state: string
  city: string
  country: string
  role: string
  profile_picture_url: string
  bio?: string
  referred_by: {
    user_id: string
    name: string
    relationship_type: string
    relationship_description?: string
  }[]
  referred_to: {  // Students/people this person referred
    user_id: string
    name: string
    relationship_type: string
    relationship_description?: string
  }[]
  uscf_id?: string
  lichess_username?: string
  chesscom_username?: string
}
```

---

## 3. API ENDPOINTS

### Authentication & Authorization

- All endpoints require **Admin role only**
- JWT token validation via existing middleware
- Add new admin permission check for referral routes

---

### 3.1 Read Endpoints

#### **GET /referral-network/graph**

Returns the complete network graph data.

**Query Parameters:**

- `state` (optional) - Filter nodes by state (returns subgraph)
- `relationship_type` (optional) - Filter edges by type
- `include_inactive` (optional, boolean) - Default: false (exclude inactive users)

**Response:**

```
{
  "success": true,
  "data": {
    "nodes": [...],
    "edges": [...],
    "metadata": {...}
  }
}
```

**Use Cases:**

- Load full network on admin dashboard
- Render D3.js/Cytoscape visualization
- Apply filters before rendering

---

#### **GET /referral-network/node/{user_id}**

Get detailed information for a specific node (used on hover).

**Response:**

```
{
  "success": true,
  "data": {
    NodeDetail object
  }
}
```

**Use Cases:**

- Tooltip/popover on node hover
- Expand node details in sidebar
- Explore individual user's referral network

---

#### **GET /referral-network/stats**

Returns aggregated statistics about the referral network.

**Query Parameters:**

- `state` (optional) - Stats for specific state only

**Response:**

```
{
  "success": true,
  "data": {
    "total_users_in_network": int,
    "total_referral_relationships": int,
    "by_state": {
      "state1": {
        "users": int,
        "referrals": int,
        "avg_referrals_per_person": float
      }
    },
    "by_relationship_type": {
      "vendor": int,
      "classmate_college": int,
      ...
    },
    "orphan_users": int,  // Users with no referral relationships
    "most_referred_person": {
      "user_id": string,
      "name": string,
      "referral_count": int
    },
    "biggest_referrer": {
      "user_id": string,
      "name": string,
      "referral_count": int
    }
  }
}
```

---

#### **GET /referral-network/relationships**

List all relationships with optional filtering.

**Query Parameters:**

- `referrer_id` (optional)
- `referee_id` (optional)
- `relationship_type` (optional)
- `state` (optional)
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

**Response:**

```
{
  "success": true,
  "data": [...],  // Array of ReferralEdge objects
  "pagination": {
    "total": int,
    "limit": int,
    "offset": int
  }
}
```

---

### 3.2 Write Endpoints

#### **POST /referral-network/node**

Add a new node (person) to the referral network.

In practice, nodes are automatically created when users join the system. This endpoint would be used to:

1. Create new users via admin panel
2. Enroll existing users into the referral network if not already in it

**Request Body:**

```
{
  "referrer_user_id": "string",  // Who referred this person (optional)
  "relationship_type": "string",  // How they know each other (optional)
  "relationship_description": "string",  // Additional context (optional)

  // OR create new user:
  "first_name": "string",
  "last_name": "string",
  "email": "string",
  "state": "string",
  "city": "string",
  "role": "string" (coach|mentor|student)
}
```

**Response:**

```
{
  "success": true,
  "data": {
    ReferralNode object + relationship info
  }
}
```

**Business Logic:**

- Validate referrer_id exists
- Validate relationship_type is in allowed enum
- Set created_by to current admin ID
- Trigger any user creation workflow if needed

---

#### **POST /referral-network/relationship**

Create a new referral relationship between existing users.

**Request Body:**

```
{
  "referrer_id": "string",  // Required. Person who referred
  "referee_id": "string",   // Required. Person who was referred
  "relationship_type": "string",  // Required. Enum value
  "relationship_description": "string"  // Optional
}
```

**Response:**

```
{
  "success": true,
  "data": {
    ReferralEdge object  // The created relationship
  }
}
```

**Business Logic:**

- Validate both users exist and are active
- Check for duplicate relationship (referrer_id, referee_id)
- Check for circular relationships (optional - decide if cycles are allowed)
- Return 409 Conflict if relationship already exists
- Set created_by to current admin ID

---

#### **PUT /referral-network/relationship/{relationship_id}**

Update an existing relationship (e.g., change relationship type or description).

**Request Body:**

```
{
  "relationship_type": "string",  // Optional
  "relationship_description": "string"  // Optional
}
```

**Response:**

```
{
  "success": true,
  "data": {
    ReferralEdge object  // Updated relationship
  }
}
```

---

#### **DELETE /referral-network/relationship/{relationship_id}**

Remove a referral relationship.

**Response:**

```
{
  "success": true,
  "message": "Relationship deleted successfully"
}
```

**Business Logic:**

- Soft delete or hard delete (decide based on audit requirements)
- Log deletion with admin ID

---

### 3.3 Search/Filter Endpoints

#### **GET /referral-network/search**

Search for nodes by name or other attributes.

**Query Parameters:**

- `q` (string) - Search term (name, email)
- `state` (optional)
- `role` (optional)
- `limit` (optional, default: 20)

**Response:**

```
{
  "success": true,
  "data": [
    ReferralNode[]
  ]
}
```

**Use Cases:**

- Typeahead/autocomplete when creating relationships
- Find users by name in UI

---

#### **GET /referral-network/connected-path**

Find the shortest path between two users (optional advanced feature).

**Query Parameters:**

- `from_user_id` (required)
- `to_user_id` (required)

**Response:**

```
{
  "success": true,
  "data": {
    "path": [
      {
        "user_id": string,
        "name": string,
        "position": int  // 0, 1, 2, etc. in path
      }
    ],
    "distance": int,
    "edges": [
      {
        "from": string,
        "to": string,
        "relationship_type": string
      }
    ]
  }
}
```

---

## 4. FRONT-END REQUIREMENTS (For Reference)

### Graph Visualization Library Options

- **D3.js** - Full control, complex but powerful
- **Cytoscape.js** - Graph-specific, easier for network viz
- **Vis.js** - Good balance, force-directed layout built-in
- **React Force Graph** - React wrapper, good for React apps

### Key Features to Implement

1. **State-based Clustering**: Force-directed layout with nodes from same state gravitating together
2. **Color Mapping**: Node colors by state or role
3. **Interactive Hover**: Show NodeDetail on hover
4. **Filtering UI**: Filter by state, relationship type
5. **Zoom/Pan**: Navigation in large graphs
6. **Legend**: Explain colors and shapes
7. **Add Node Form**: Modal/sidebar to add relationships
8. **Statistics Panel**: Show network metrics

---

## 5. PERMISSION & AUTHORIZATION

### Endpoint Access Control

All endpoints protected by:

- `@RequireAuth()` - Must be logged in
- `@RequireAdmin()` - Must be admin role

Added to existing auth middleware pattern:

```
r.Route("/referral-network", func(r chi.Router) {
    r.Use(auth.AuthMiddleware(ss.Store))
    r.Use(isAdminMiddleware)  // NEW

    r.Get("/graph", referralH.GetGraph)
    r.Post("/relationship", referralH.CreateRelationship)
    // ... etc
})
```

---

## 6. IMPLEMENTATION LAYERS

### A. Database Layer (Store)

New file: `internal/store/referralstore.go`

- Methods to CRUD referral_relationships table
- Query methods for graph traversal
- Relationship validation queries

```
interface ReferralStore {
  CreateRelationship(ctx, referrer_id, referee_id, type, description) error
  GetRelationship(ctx, relationship_id) (*Relationship, error)
  GetUserRelationships(ctx, user_id) ([]*Relationship, error)
  GetFullGraph(ctx, filters) (*Graph, error)
  UpdateRelationship(ctx, relationship_id, updates) error
  DeleteRelationship(ctx, relationship_id) error
  SearchUsers(ctx, query, filters) ([]*User, error)
  // ... graph analytics queries
}
```

### B. Service Layer

New file: `internal/service/referral.go`

- Business logic for referral operations
- Complex graph queries (shortest path, clustering)
- Validation and authorization
- Response object construction

```
interface ReferralService {
  AddNodeToNetwork(ctx, userID, referrerID, relationType) error
  ConnectUsers(ctx, referrerID, refereeID, relationType) error
  GetNetworkGraph(ctx, filters) (*ReferralGraph, error)
  GetNodeDetail(ctx, userID) (*NodeDetail, error)
  GetNetworkStats(ctx, filters) (*NetworkStats, error)
  // ... analytics methods
}
```

### C. Handler Layer

New file: `internal/api/v1/referral_handlers.go`

- HTTP request/response handling
- Parameter validation
- Error handling
- Auth middleware integration

```
type ReferralHandler struct {
  referralService *service.ReferralService
  store           *store.Store
}

Methods:
- GetGraph(w, r)
- GetNodeDetail(w, r)
- GetStats(w, r)
- CreateRelationship(w, r)
- UpdateRelationship(w, r)
- DeleteRelationship(w, r)
- SearchUsers(w, r)
- GetRelationships(w, r)
```

---

## 7. ERROR HANDLING

Standard HTTP responses:

- **200 OK** - Success
- **201 Created** - Resource created
- **400 Bad Request** - Invalid input (missing fields, invalid types)
- **401 Unauthorized** - Not authenticated
- **403 Forbidden** - Not admin
- **404 Not Found** - User/relationship doesn't exist
- **409 Conflict** - Relationship already exists / circular ref
- **500 Internal Server Error** - Database/server error

Every error response:

```
{
  "success": false,
  "message": "User friendly error message",
  "error": "Error code or details"
}
```

---

## 8. PERFORMANCE CONSIDERATIONS

### Query Optimization

1. **Graph Endpoint**: Can be expensive for large networks
   - Add pagination/state filtering to reduce data
   - Consider GraphQL for client-side filtering
   - Cache graph data with 5-10 minute TTL
   - Use database indexes on (referrer_id, referee_id)

2. **Node Detail Endpoint**:
   - N+1 problem: Get user, then 2 queries for referred_by & referred_to
   - Solution: Single query with JOINs + post-processing, OR GraphQL

3. **Statistics Endpoint**:
   - Aggregate queries over entire table
   - Consider materialized views or cache for frequently accessed stats

### Sample Indexes

```sql
CREATE INDEX idx_referral_referrer ON referral_relationships(referrer_id);
CREATE INDEX idx_referral_referee ON referral_relationships(referee_id);
CREATE INDEX idx_referral_type ON referral_relationships(relationship_type);
CREATE UNIQUE INDEX idx_referral_unique ON referral_relationships(referrer_id, referee_id);
```

---

## 9. MIGRATION STRATEGY

### SQL Migration File: `0010_create_referral_relationships.up.sql`

```sql
CREATE TYPE relationship_type AS ENUM (
  'vendor',
  'classmate_college',
  'classmate_school',
  'coworker',
  'family',
  'friend',
  'coach',
  'student',
  'other'
);

CREATE TABLE referral_relationships (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  referrer_id VARCHAR(10) NOT NULL REFERENCES users(id),
  referee_id VARCHAR(10) NOT NULL REFERENCES users(id),
  relationship_type relationship_type NOT NULL,
  relationship_description TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  created_by VARCHAR(10) REFERENCES users(id),

  CONSTRAINT no_self_referral CHECK (referrer_id != referee_id),
  CONSTRAINT unique_relationship UNIQUE (referrer_id, referee_id)
);

CREATE INDEX idx_referral_referrer ON referral_relationships(referrer_id);
CREATE INDEX idx_referral_referee ON referral_relationships(referee_id);
CREATE INDEX idx_referral_type ON referral_relationships(relationship_type);
```

---

## 10. OPTIONAL ADVANCED FEATURES (Future)

1. **Graph Analytics**
   - Clustering coefficient
   - Betweenness centrality
   - Community detection
   - Network diameter

2. **Visualization Enhancements**
   - 3D graph rendering
   - Time-based animation (show growth over time)
   - Export as image/PDF

3. **Relationship Analytics**
   - Most effective referral source
   - Conversion funnel from referral to coach

4. **Bulk Operations**
   - Import relationships from CSV
   - Batch create relationships

---

## 11. TESTING STRATEGY

### Unit Tests

- Service layer: Relationship logic, validation
- Query logic: Filter combinations, edge cases

### Integration Tests

- Create relationship ΓåÆ verify in queryable state
- Delete relationship ΓåÆ verify removed from graph
- Node detail hover info ΓåÆ verify complete data

### API Tests

- CRUD operations on relationships
- Filter/search functionality
- Permission checks (admin-only)
- Error cases (duplicates, self-referral)

---

## Summary

| Component           | What It Does                                        |
| ------------------- | --------------------------------------------------- |
| **DB Table**        | Stores referrer-referee relationships with metadata |
| **Service Layer**   | Graph queries, validation, business logic           |
| **Store Layer**     | Raw CRUD operations, complex SQL queries            |
| **Handler Layer**   | HTTP endpoints, request validation                  |
| **Auth**            | Admin-only access via existing middleware           |
| **Response Models** | Node, Edge, Graph, NodeDetail for FE                |
| **Performance**     | Indexed queries, optional caching, filtering        |

This plan maintains your existing architecture (GORM ΓåÆ Service ΓåÆ Handler) and fits naturally into the current API structure.
