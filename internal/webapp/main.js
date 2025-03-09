/**
 * Template Plugin Frontend
 * 
 * This module provides the interactive user interface for managing items in the template plugin.
 * It handles:
 * - Loading and displaying items with pagination
 * - Creating new items
 * - Searching existing items
 * - Basic error handling
 * - UI state management
 */

/**
 * Global State Management
 * Tracks the current page and items per page for pagination
 */
let currentPage = 1;
const itemsPerPage = 10;

/**
 * API Interaction Functions
 * These functions handle all communication with the backend API
 */
/**
 * Loads a page of items from the API
 * @param {number} page - The page number to load (defaults to 1)
 * @returns {Promise<void>}
 */
async function loadItems(page = 1) {
    try {
        const response = await fetch(`/api/items?page=${page}&limit=${itemsPerPage}`);
        if (!response.ok) throw new Error('Failed to load items');
        
        const data = await response.json();
        renderItems(data);
        renderPagination(data);
        currentPage = page;
    } catch (error) {
        console.error('Error loading items:', error);
        // TODO: Show user-friendly error message
    }
}

/**
 * Creates a new item via the API
 * @param {string} name - The name of the new item
 * @param {string} description - The description of the new item
 * @returns {Promise<void>}
 */
async function createItem(name, description) {
    try {
        const response = await fetch('/api/items', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ name, description }),
        });
        
        if (!response.ok) throw new Error('Failed to create item');
        
        await loadItems(currentPage); // Refresh the current page
    } catch (error) {
        console.error('Error creating item:', error);
        // TODO: Show user-friendly error message
    }
}

/**
 * Searches for items matching the given query
 * @param {string} query - The search query
 * @returns {Promise<void>}
 */
async function searchItems(query) {
    try {
        const response = await fetch(`/api/items/search?q=${encodeURIComponent(query)}`);
        if (!response.ok) throw new Error('Search failed');
        
        const data = await response.json();
        renderItems(data);
        document.getElementById('pagination').style.display = 'none';
    } catch (error) {
        console.error('Error searching items:', error);
        // TODO: Show user-friendly error message
    }
}

/**
 * UI Rendering Functions
 * These functions handle updating the DOM with new data
 */

/**
 * Renders the list of items to the page
 * @param {Object} data - The data containing items to render
 */
function renderItems(data) {
    const itemsDiv = document.getElementById('items');
    if (!data.items.length) {
        itemsDiv.innerHTML = '<p>No items found</p>';
        return;
    }
    
    itemsDiv.innerHTML = data.items.map(item => `
        <div class="item">
            <h3>${escapeHtml(item.name)}</h3>
            <p>${escapeHtml(item.description || '')}</p>
            <button onclick="deleteItem(${item.id})">Delete</button>
        </div>
    `).join('');
}

/**
 * Renders the pagination controls
 * @param {Object} data - The data containing pagination information
 */
function renderPagination(data) {
    const totalPages = Math.ceil(data.total / data.limit);
    const paginationDiv = document.getElementById('pagination');
    
    if (totalPages <= 1) {
        paginationDiv.style.display = 'none';
        return;
    }
    
    paginationDiv.style.display = 'block';
    paginationDiv.innerHTML = `
        <button ${data.page <= 1 ? 'disabled' : ''} 
                onclick="loadItems(${data.page - 1})">Previous</button>
        <span>Page ${data.page} of ${totalPages}</span>
        <button ${data.page >= totalPages ? 'disabled' : ''} 
                onclick="loadItems(${data.page + 1})">Next</button>
    `;
}

/**
 * Utility function to escape HTML special characters
 * @param {string} unsafe - The string to escape
 * @returns {string} The escaped string
 */
function escapeHtml(unsafe) {
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

/**
 * Event Handler Setup
 * Initializes all event listeners and form handlers when the DOM is ready
 * - Loads initial items
 * - Sets up search functionality
 * - Sets up item creation form
 */
document.addEventListener('DOMContentLoaded', () => {
    // Load initial items on page load
    loadItems(1);

    // Setup search form handler
    const searchForm = document.getElementById('searchForm');
    searchForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const query = document.getElementById('searchInput').value;
        if (query) {
            searchItems(query);
        } else {
            loadItems(1); // Reset to first page if search is cleared
        }
    });

    // Setup item creation form handler
    const createForm = document.getElementById('createForm');
    createForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const name = document.getElementById('nameInput').value;
        const description = document.getElementById('descriptionInput').value;
        createItem(name, description);
        createForm.reset(); // Clear form after submission
    });
});
