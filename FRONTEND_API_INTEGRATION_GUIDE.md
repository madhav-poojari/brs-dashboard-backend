# Referral Network API - Frontend Integration Guide

## Overview
5 new endpoints to support the **referral network graph visualization**. All endpoints are **admin-only** and require JWT authentication.

---

## Endpoints

### 1. **GET /referral-network/graph**
**Purpose:** Fetch complete network data for visualization

**Query Parameters:**
- `state` (optional) - Filter nodes by state (e.g., `?state=CA`)

**Response:**
```json
{
  "success": true,
  "message": "Network graph fetched successfully",
  "data": {
    "nodes": [
      {
        "id": "USR00WUFKO",
        "name": "John Doe",
        "state": "CA",
        "city": "San Francisco",
        "role": "coach",
        "profile_picture_url": "https://..."
      }
    ],
    "edges": [
      {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "source": "USR00WUFKO",
        "target": "USR00LHIY8",
        "relationship_type": "vendor",
        "relationship_description": null,
        "created_at": "2026-04-08T10:30:00Z"
      }
    ],
    "metadata": {
      "total_nodes": 45,
      "total_edges": 120
    }
  }
}
```

**Frontend Use:**
- Load this on initial page load
- Pass to D3.js/Cytoscape.js/vis.js for visualization
- Apply force-directed layout (library handles coordinates)
- Color nodes by state
- State filter to focus on specific regions

**Performance Note:** Can load all data; add pagination if >1000 nodes

---

### 2. **GET /referral-network/node/{user_id}**
**Purpose:** Get detailed info for a specific node (used on hover/tooltip)

**Example:**
```
GET /referral-network/node/USR00WUFKO
```

**Response:**
```json
{
  "success": true,
  "message": "Node detail fetched successfully",
  "data": {
    "user_id": "USR00WUFKO",
    "full_name": "John Doe",
    "email": "john@example.com",
    "state": "CA",
    "city": "San Francisco",
    "country": "USA",
    "role": "coach",
    "profile_picture_url": "https://...",
    "bio": "Chess coach with 10 years experience",
    "personal_meet_link": "https://meet.com/...",
    "uscf_id": "12345678",
    "lichess_username": "johndoe",
    "chesscom_username": "johndoe",
    "referred_by": [
      {
        "user_id": "USR001234",
        "name": "Jane Smith",
        "relationship_type": "vendor",
        "relationship_description": null
      }
    ],
    "referred_to": [
      {
        "user_id": "USR005678",
        "name": "Bob Johnson",
        "relationship_type": "classmate_college",
        "relationship_description": "College chess club mate"
      }
    ]
  }
}
```

**Frontend Use:**
- Show on node hover (tooltip/popover)
- Click to expand sidebar with full details
- Display profile picture, bio, chess usernames
- Show "Referred by" and "Referred to" lists

---

### 3. **POST /referral-network/relationship**
**Purpose:** Create new referral relationship between two users

**Request Body:**
```json
{
  "referrer_id": "USR00WUFKO",
  "referee_id": "USR00LHIY8",
  "relationship_type": "vendor",
  "relationship_description": "Met at chess tournament" 
}
```

**Relationship Types (enum):**
- `vendor`
- `classmate_college`
- `classmate_school`
- `coworker`
- `family`
- `friend`
- `coach`
- `student`
- `other`

**Response (Success):**
```json
{
  "success": true,
  "message": "Relationship created successfully",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "referrer_id": "USR00WUFKO",
    "referee_id": "USR00LHIY8",
    "relationship_type": "vendor",
    "relationship_description": "Met at chess tournament",
    "created_at": "2026-04-08T10:30:00Z"
  }
}
```

**Error Responses:**
- `400` - Self-referral, invalid type, missing fields
- `409` - Relationship already exists
- `500` - Server error

**Frontend Use:**
- "Add Node" form/modal
- Dropdown for relationship_type
- Textarea for optional description
- After success: refresh graph ΓåÆ `GET /referral-network/graph`

---

### 4. **PUT /referral-network/relationship/{relationship_id}**
**Purpose:** Update relationship metadata (type or description)

**Example:**
```
PUT /referral-network/relationship/550e8400-e29b-41d4-a716-446655440000
```

**Request Body (both optional):**
```json
{
  "relationship_type": "classmate_college",
  "relationship_description": "College friend from chess club"
}
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Relationship updated successfully"
}
```

**Frontend Use:**
- Edit modal for relationship details
- Click relationship edge ΓåÆ show edit dialog
- Update type/description
- After success: refresh graph

---

### 5. **DELETE /referral-network/relationship/{relationship_id}**
**Purpose:** Remove a referral relationship

**Example:**
```
DELETE /referral-network/relationship/550e8400-e29b-41d4-a716-446655440000
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Relationship deleted successfully"
}
```

**Frontend Use:**
- Right-click on edge ΓåÆ "Delete relationship"
- Confirm dialog before deletion
- After success: refresh graph
- Visual feedback (edge disappears)

---

## Authentication
All endpoints require:
```
Authorization: Bearer {jwt_token}
```

Admin role is enforced server-side. If non-admin tries to access:
```json
{
  "success": false,
  "message": "Forbidden"
}
```

---

## Suggested Frontend Workflow

### Initial Load
```javascript
1. GET /referral-network/graph
2. Pass nodes + edges to visualization library
3. Library calculates layout + renders
4. Add event listeners to nodes
```

### Node Hover
```javascript
1. User hovers over node
2. GET /referral-network/node/{user_id}
3. Display tooltip with full details
```

### Add Relationship
```javascript
1. Open "Add Relationship" modal
2. User selects referrer + referee from dropdowns
3. User selects relationship_type
4. User enters optional description
5. POST /referral-network/relationship
6. Success ΓåÆ GET /referral-network/graph (refresh visualization)
```

### Edit Relationship
```javascript
1. User clicks on edge or relationship card
2. Open edit modal with current values
3. User updates type/description
4. PUT /referral-network/relationship/{id}
5. Success ΓåÆ GET /referral-network/graph (refresh)
```

### Delete Relationship
```javascript
1. User clicks delete on relationship
2. Confirm dialog
3. DELETE /referral-network/relationship/{id}
4. Success ΓåÆ GET /referral-network/graph (refresh)
```

---

## Data Model Reference

### Node (from graph endpoint)
```typescript
interface Node {
  id: string;
  name: string;
  state: string;
  city: string;
  role: string;
  profile_picture_url: string;
}
```

### Edge (from graph endpoint)
```typescript
interface Edge {
  id: string;
  source: string; // user_id of referrer
  target: string; // user_id of referee
  relationship_type: string;
  relationship_description: string | null;
  created_at: string; // ISO timestamp
}
```

### Node Detail (from node endpoint)
```typescript
interface NodeDetail {
  user_id: string;
  full_name: string;
  email: string;
  state: string;
  city: string;
  country: string;
  role: string;
  profile_picture_url: string;
  bio: string;
  personal_meet_link: string;
  uscf_id: string;
  lichess_username: string;
  chesscom_username: string;
  referred_by: Array<{
    user_id: string;
    name: string;
    relationship_type: string;
    relationship_description: string | null;
  }>;
  referred_to: Array<{
    user_id: string;
    name: string;
    relationship_type: string;
    relationship_description: string | null;
  }>;
}
```

---

## Visualization Tips

### State-based Clustering
- Use vis.js `physics` option with custom node groups by state
- Or D3.js `forceSimulation()` with added "state" force

### Color Coding
- Map states to colors: `{ CA: '#FF6B6B', TX: '#4ECDC4', ... }`
- Or map by role: `{ coach: '#FF6B6B', mentor: '#4ECDC4', student: '#95E1D3' }`

### Zoom/Pan
- Add zoom + pan controls (most libraries support this natively)
- Reset button to fit all nodes in view

### Legend
- Show color meanings
- Show relationship type symbols/styles

### Edge Weight
- Optional: thicker lines for common relationship types
- Optional: curved edges to show direction (referrer ΓåÆ referee)

---

## Error Handling

### Network Errors
```json
{
  "success": false,
  "message": "Failed to fetch network graph",
  "error": "Connection timeout"
}
```

### Validation Errors
```json
{
  "success": false,
  "message": "referrer_id, referee_id, and relationship_type are required"
}
```

### Authorization Errors
```json
{
  "success": false,
  "message": "Unauthorized"
}
```

---

## Rate Limiting
None currently. Implement if needed for large graphs (>5000 nodes).

---

## Future Endpoints (Not Implemented)
- `GET /referral-network/stats` - Network statistics by state/type
- `GET /referral-network/search?q=name` - Search users for dropdown
- `GET /referral-network/connected-path?from={id}&to={id}` - Shortest path between users

