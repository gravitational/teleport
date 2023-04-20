from langchain.document_loaders import DirectoryLoader
from langchain.document_loaders import TextLoader
from langchain.indexes import VectorstoreIndexCreator
from langchain.vectorstores import Chroma
from langchain.embeddings import OpenAIEmbeddings
from langchain.text_splitter import TokenTextSplitter

loader = DirectoryLoader('./knowledge', loader_cls=TextLoader)
index_builder = VectorstoreIndexCreator(
    vectorstore_cls=Chroma, 
    embedding=OpenAIEmbeddings(),
    text_splitter=TokenTextSplitter(chunk_size=500, chunk_overlap=20)
)
index = index_builder.from_loaders([loader])
retreiver = index.vectorstore.as_retriever()
